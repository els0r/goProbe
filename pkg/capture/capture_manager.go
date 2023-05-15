package capture

import (
	"context"
	"sync"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/els0r/goProbe/pkg/logging"
	"golang.org/x/exp/slog"
)

// Manager manages a set of Capture instances.
// Each interface can be associated with up to one Capture.
type Manager struct {
	sync.RWMutex

	writeoutHandler writeout.Handler
	captures        map[string]*Capture
	sourceInitFn    sourceInitFn
	lastRotation    time.Time
}

// NewManager creates a new Manager
func NewManager(ctx context.Context, writeoutHandler writeout.Handler) *Manager {
	return &Manager{
		captures:        make(map[string]*Capture),
		writeoutHandler: writeoutHandler,
		sourceInitFn:    defaultSourceInitFn,
	}
}

func (cm *Manager) LastRotation() (t time.Time) {
	cm.RLock()
	t = cm.lastRotation
	cm.RUnlock()

	return
}

func (cm *Manager) ScheduleWriteouts(ctx context.Context, interval time.Duration) {

	go func() {
		logger := logging.FromContext(ctx)

		// wait until the next 5 minute interval of the hour is reached before starting the ticker
		tNow := time.Now()

		sleepUntil := tNow.Truncate(interval).Add(interval).Sub(tNow)
		logger.Infof("waiting for %s to start capture rotation", sleepUntil.Round(time.Second))

		timer := time.NewTimer(sleepUntil)
		select {
		case <-timer.C:
			break
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(interval)

		// immediately write out after the initial sleep has completed
		t := time.Now()
		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping rotation handler")
				return
			default:

				cm.performWriteout(ctx, t)
				// 	if len(h.writeoutsChan) > 2 {
				// 		log := logger.With("queue_length", len(h.writeoutsChan))
				// 		if len(h.writeoutsChan) > capture.WriteoutsChanDepth {
				// 			log.Fatalf("writeouts are lagging behind too much")
				// 		}
				// 		log.Warn("writeouts are lagging behind")
				// 	}

				// wait for the the next ticker to complete
				t = <-ticker.C
			}
		}
	}()

}

// SetSourceInitFn sets a custom function used to initialize a new capture
func (cm *Manager) SetSourceInitFn(fn sourceInitFn) *Manager {
	cm.sourceInitFn = fn
	return cm
}

// Status fetches the current capture stats from all (or a set of) interfaces
func (cm *Manager) Status(ctx context.Context, ifaces ...string) map[string]capturetypes.CaptureStats {

	statusmapMutex := sync.Mutex{}
	statusmap := make(map[string]capturetypes.CaptureStats)

	var rg RunGroup
	cmCopy := cm.capturesCopy()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	ifaces = buildIfaces(cmCopy, ifaces...)

	for _, iface := range ifaces {
		iface := iface
		mc, exists := cm.captures[iface]
		if exists {
			rg.Run(func() {

				runCtx := logging.WithFields(ctx, slog.String("iface", iface))
				logger := logging.FromContext(runCtx)

				// Lock the running capture and extract the status
				mc.lock()

				// Since the capture is locked we can safely extract the (capture) status
				// from the individual interfaces (and unlock no matter what)
				status, err := mc.status()
				mc.unlock()

				if err != nil {
					logger.Errorf("failed to get capture stats: %v", err)
					return
				}

				statusmapMutex.Lock()
				statusmap[iface] = *status
				statusmapMutex.Unlock()
			})
		}
	}
	rg.Wait()

	return statusmap
}

// TODO: Ensure Update() and Close() cannot be called in parallel
func (cm *Manager) Update(ctx context.Context, ifaces config.Ifaces) {

	logger := logging.FromContext(ctx)
	t0 := time.Now()

	cmCopy := cm.capturesCopy()

	ifaceSet := make(map[string]struct{})
	var enableIfaces []string
	for iface := range ifaces {
		ifaceSet[iface] = struct{}{}

		if _, exists := cmCopy[iface]; !exists {
			enableIfaces = append(enableIfaces, iface)
		}
	}

	var rg RunGroup
	for _, iface := range enableIfaces {
		iface := iface
		rg.Run(func() {

			runCtx := logging.WithFields(ctx, slog.String("iface", iface))
			logger := logging.FromContext(runCtx)

			cap := newCapture(iface, ifaces[iface]).SetSourceInitFn(cm.sourceInitFn)
			if err := cap.run(runCtx); err != nil {
				logger.Errorf("failed to start capture: %s", err)
				return
			}
			cm.Lock()
			cm.captures[iface] = cap
			cm.Unlock()
		})
	}
	rg.Wait()

	// Contains the names of all interfaces we are shutting down and deleting.
	var disableIfaces []string
	for iface := range cmCopy {
		if _, exists := ifaceSet[iface]; !exists {
			disableIfaces = append(disableIfaces, iface)
		}
	}
	if len(disableIfaces) > 0 {
		cm.Close(ctx, disableIfaces...)
	}

	logger.With("elapsed", time.Since(t0).Round(time.Millisecond).String()).Debug("updated interface list")
}

func (cm *Manager) Close(ctx context.Context, ifaces ...string) {

	// Execute a final writeout of all interfaces
	cm.performWriteout(ctx, time.Now().Add(time.Second), ifaces...)

	var rg RunGroup
	cmCopy := cm.capturesCopy()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	ifaces = buildIfaces(cmCopy, ifaces...)

	// Close all interfaces
	for _, iface := range ifaces {
		iface := iface
		mc, exists := cmCopy[iface]
		if exists {
			rg.Run(func() {
				runCtx := logging.WithFields(ctx, slog.String("iface", iface))
				logger := logging.FromContext(runCtx)

				if err := mc.close(); err != nil {
					logger.Errorf("failed to close capture: %s", err)
					return
				}
				cm.Lock()
				delete(cm.captures, iface)
				cm.Unlock()
			})
		}
	}
	rg.Wait()
}

func (cm *Manager) rotate(ctx context.Context, writeoutChan chan<- capturetypes.TaggedAggFlowMap, ifaces ...string) {

	var rg RunGroup
	cmCopy := cm.capturesCopy()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	ifaces = buildIfaces(cmCopy, ifaces...)

	for _, iface := range ifaces {
		iface := iface
		mc, exists := cm.captures[iface]
		if exists {
			rg.Run(func() {

				runCtx := logging.WithFields(ctx, slog.String("iface", iface))
				logger := logging.FromContext(runCtx)

				// Lock the running capture and perform the rotation
				mc.lock()

				rotateResult := mc.rotate(runCtx)

				// Since the capture is locked we can safely extract the (capture) status
				// from the individual interfaces (and unlock no matter what)
				stats, err := mc.status()
				if err != nil {
					logger.Errorf("failed to get capture stats: %v", err)
				}
				mc.unlock()

				writeoutChan <- capturetypes.TaggedAggFlowMap{
					Map:   rotateResult,
					Stats: *stats,
					Iface: iface,
				}
			})
		}
	}
	rg.Wait()
}

func (cm *Manager) performWriteout(ctx context.Context, timestamp time.Time, ifaces ...string) {
	writeoutChan := make(chan capturetypes.TaggedAggFlowMap, writeout.WriteoutsChanDepth)
	doneChan := cm.writeoutHandler.HandleWriteout(ctx, timestamp, writeoutChan)

	cm.rotate(ctx, writeoutChan, ifaces...)
	close(writeoutChan)

	<-doneChan

	cm.Lock()
	cm.lastRotation = timestamp
	cm.Unlock()
}

func (cm *Manager) capturesCopy() map[string]*Capture {
	copyMap := make(map[string]*Capture)

	cm.Lock()
	for iface, capture := range cm.captures {
		copyMap[iface] = capture
	}
	cm.Unlock()

	return copyMap
}

func buildIfaces(captures map[string]*Capture, ifaces ...string) []string {
	if len(ifaces) == 0 {
		ifaces = make([]string, 0, len(captures))
		for iface := range captures {
			ifaces = append(ifaces, iface)
		}
	}

	return ifaces
}
