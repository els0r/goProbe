package capture

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/els0r/goProbe/pkg/logging"
	"golang.org/x/exp/slog"
)

const allowedWriteoutDurationFraction = 0.1

// Manager manages a set of Capture instances.
// Each interface can be associated with up to one Capture.
type Manager struct {
	sync.RWMutex

	writeoutHandler writeout.Handler
	captures        map[string]*Capture
	sourceInitFn    sourceInitFn

	lastRotation time.Time
}

// InitManager initializes a CaptureManager and the underlying writeout logic
// Used as primary entrypoint for the goProbe binary and E2E tests
func InitManager(ctx context.Context, config *config.Config, opts ...ManagerOption) (*Manager, error) {

	// Setup database compression and permissions
	encoderType, err := encoders.GetTypeByString(config.DB.EncoderType)
	if err != nil {
		return nil, fmt.Errorf("failed to get encoder type from %s: %w", config.DB.EncoderType, err)
	}
	dbPermissions := goDB.DefaultPermissions
	if config.DB.Permissions != 0 {
		dbPermissions = config.DB.Permissions
	}

	// Initialize the DB writeout handler
	writeoutHandler := writeout.NewGoDBHandler(config.DB.Path, encoderType).
		WithSyslogWriting(config.SyslogFlows).
		WithPermissions(dbPermissions)

		// Initialize the CaptureManager
	captureManager := NewManager(writeoutHandler, opts...)

	// Update (i.e. start) all capture routines (implicitly by reloading all configurations) and schedule
	// DB writeouts
	captureManager.Update(ctx, config.Interfaces)
	captureManager.ScheduleWriteouts(ctx, time.Duration(goDB.DBWriteInterval)*time.Second)

	return captureManager, nil
}

// NewManager creates a new CaptureManager
func NewManager(writeoutHandler writeout.Handler, opts ...ManagerOption) *Manager {
	captureManager := &Manager{
		captures:        make(map[string]*Capture),
		writeoutHandler: writeoutHandler,
		sourceInitFn:    defaultSourceInitFn,
	}
	for _, opt := range opts {
		opt(captureManager)
	}
	return captureManager
}

// LastRotation returns the timestamp of the last DB writeout / rotation
func (cm *Manager) LastRotation() (t time.Time) {
	cm.RLock()
	t = cm.lastRotation
	cm.RUnlock()

	return
}

// ScheduleWriteouts creates a new goroutine that executes a DB writeout in defined time
// intervals
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

				t0 := time.Now()
				cm.performWriteout(ctx, t)
				if elapsed := float64(time.Since(t0)); elapsed > allowedWriteoutDurationFraction*float64(interval) {
					logger.Warnf("writeouts took longer than %.1f%% of the writeout interval (%.1f%%)",
						100*allowedWriteoutDurationFraction,
						100.*elapsed/float64(interval))
				}

				// wait for the the next ticker to complete
				t = <-ticker.C
			}
		}
	}()
}

// ManagerOption denotes a functional option for any CaptureManager
type ManagerOption func(cm *Manager)

// WithSourceInitFn sets a custom function used to initialize a new capture
func WithSourceInitFn(fn sourceInitFn) ManagerOption {
	return func(cm *Manager) {
		cm.sourceInitFn = fn
	}
}

// Status fetches the current capture stats from all (or a set of) interfaces
func (cm *Manager) Status(ctx context.Context, ifaces ...string) (statusmap map[string]capturetypes.CaptureStats) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	statusmap = make(map[string]capturetypes.CaptureStats)

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	cmCopy := cm.capturesCopy()
	if ifaces = buildIfaces(cmCopy, ifaces...); len(ifaces) == 0 {
		return
	}

	var (
		statusmapMutex = sync.Mutex{}
		rg             RunGroup
	)
	for _, iface := range ifaces {
		iface := iface
		mc, exists := cm.captures[iface]
		if exists {
			rg.Run(func() {

				runCtx := logging.WithFields(ctx, slog.String("iface", iface))

				// Lock the running capture and extract the status
				mc.lock()

				// Since the capture is locked we can safely extract the (capture) status
				// from the individual interfaces (and unlock no matter what)
				status, err := mc.status()
				mc.unlock()

				if err != nil {
					logging.FromContext(runCtx).Errorf("failed to get capture stats: %v", err)
					return
				}

				statusmapMutex.Lock()
				statusmap[iface] = *status
				statusmapMutex.Unlock()
			})
		}
	}
	rg.Wait()

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Debug("retrieved interface status")

	return
}

// Update the configuration for all (or a set of) interfaces
// TODO: Ensure Update() and Close() cannot be called in parallel
func (cm *Manager) Update(ctx context.Context, ifaces config.Ifaces) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	cmCopy := cm.capturesCopy()

	// Build set of interfaces to enable / disable
	var (
		ifaceSet                    = make(map[string]struct{})
		enableIfaces, disableIfaces []string
	)
	for iface := range ifaces {
		ifaceSet[iface] = struct{}{}
		if _, exists := cmCopy[iface]; !exists {
			enableIfaces = append(enableIfaces, iface)
		}
	}
	for iface := range cmCopy {
		if _, exists := ifaceSet[iface]; !exists {
			disableIfaces = append(disableIfaces, iface)
		}
	}

	// Enable any interfaces present in the positive list
	var rg RunGroup
	for _, iface := range enableIfaces {
		iface := iface
		rg.Run(func() {

			runCtx := logging.WithFields(ctx, slog.String("iface", iface))

			cap := newCapture(iface, ifaces[iface]).SetSourceInitFn(cm.sourceInitFn)
			if err := cap.run(runCtx); err != nil {
				logging.FromContext(runCtx).Errorf("failed to start capture: %s", err)
				return
			}

			cm.Lock()
			cm.captures[iface] = cap
			cm.Unlock()
		})
	}
	rg.Wait()

	// Disable any interfaces present in the negative list
	if len(disableIfaces) > 0 {
		cm.Close(ctx, disableIfaces...)
	}

	logger.With("elapsed", time.Since(t0).Round(time.Millisecond).String()).Debug("updated interface list")
	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces_added", enableIfaces,
		"ifaces_removed", disableIfaces,
	).Debug("updated interface configuration")
}

// Close stops / closes all (or a set of) interfaces
func (cm *Manager) Close(ctx context.Context, ifaces ...string) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	cmCopy := cm.capturesCopy()
	if ifaces = buildIfaces(cmCopy, ifaces...); len(ifaces) == 0 {
		return
	}

	// Execute a final writeout of all interfaces in the list
	cm.performWriteout(ctx, time.Now().Add(time.Second), ifaces...)

	// Close all interfaces in the list
	var rg RunGroup
	for _, iface := range ifaces {
		iface := iface
		mc, exists := cmCopy[iface]
		if exists {
			rg.Run(func() {
				runCtx := logging.WithFields(ctx, slog.String("iface", iface))

				if err := mc.close(); err != nil {
					logging.FromContext(runCtx).Errorf("failed to close capture: %s", err)
					return
				}

				cm.Lock()
				delete(cm.captures, iface)
				cm.Unlock()
			})
		}
	}
	rg.Wait()

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Debug("closed interfaces")
}

func (cm *Manager) rotate(ctx context.Context, writeoutChan chan<- capturetypes.TaggedAggFlowMap, ifaces ...string) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	cmCopy := cm.capturesCopy()
	if ifaces = buildIfaces(cmCopy, ifaces...); len(ifaces) == 0 {
		return
	}

	var rg RunGroup
	for _, iface := range ifaces {
		iface := iface
		mc, exists := cmCopy[iface]
		if exists {
			rg.Run(func() {

				runCtx := logging.WithFields(ctx, slog.String("iface", iface))

				// Lock the running capture and perform the rotation
				mc.lock()

				rotateResult := mc.rotate(runCtx)

				// Since the capture is locked we can safely extract the (capture) status
				// from the individual interfaces (and unlock no matter what)
				stats, err := mc.status()
				if err != nil {
					logging.FromContext(runCtx).Errorf("failed to get capture stats: %v", err)
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

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Debug("rotated interfaces")
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
