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
	captures        *captures
	sourceInitFn    sourceInitFn

	lastAppliedConfig config.Ifaces

	lastRotation time.Time
	startedAt    time.Time
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
	_, _, _, err = captureManager.Update(ctx, config.Interfaces)
	if err != nil {
		return nil, err
	}

	// this is the first time the capture manager is started and is important to report program runtime
	captureManager.startedAt = time.Now()

	captureManager.ScheduleWriteouts(ctx, time.Duration(goDB.DBWriteInterval)*time.Second)

	return captureManager, nil
}

// NewManager creates a new CaptureManager
func NewManager(writeoutHandler writeout.Handler, opts ...ManagerOption) *Manager {
	captureManager := &Manager{
		captures:        newCaptures(),
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

// StartedAt returns the timestamp when the capture manager was initialized
func (cm *Manager) StartedAt() (t time.Time) {
	cm.RLock()
	t = cm.startedAt
	cm.RUnlock()

	return
}

// GetTimestamps is a combination of LastRotation() and StartedAt(). It exists to save a lock
// in case both timestamps are requested
func (cm *Manager) GetTimestamps() (startedAt, lastRotation time.Time) {
	cm.RLock()
	startedAt = cm.startedAt
	lastRotation = cm.lastRotation
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

// Config returns the runtime config of the capture manager for all (or a set of) interfaces
func (cm *Manager) Config(ctx context.Context, ifaces ...string) (ifaceConfigs config.Ifaces) {
	cm.RLock()
	defer cm.RUnlock()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	if ifaces = cm.captures.Ifaces(ifaces...); len(ifaces) == 0 {
		return
	}

	ifaceConfigs = make(config.Ifaces)

	for _, iface := range ifaces {
		cfg, exists := cm.lastAppliedConfig[iface]
		if exists {
			ifaceConfigs[iface] = cfg
		}
	}
	return
}

// Status fetches the current capture stats from all (or a set of) interfaces
func (cm *Manager) Status(ctx context.Context, ifaces ...string) (statusmap capturetypes.InterfaceStats) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	statusmap = make(capturetypes.InterfaceStats)

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	if ifaces = cm.captures.Ifaces(ifaces...); len(ifaces) == 0 {
		return
	}

	var (
		statusmapMutex = sync.Mutex{}
		rg             RunGroup
	)
	for _, iface := range ifaces {
		mc, exists := cm.captures.Get(iface)
		if !exists {
			continue
		}
		rg.Run(func() {

			runCtx := withIfaceContext(ctx, mc.iface)

			// Lock the running capture and extract the status
			mc.lock(false)

			// Since the capture is locked we can safely extract the (capture) status
			// from the individual interfaces (and unlock no matter what)
			status, err := mc.status()
			mc.unlock()

			if err != nil {
				logging.FromContext(runCtx).Errorf("failed to get capture stats: %v", err)
				return
			}

			statusmapMutex.Lock()
			statusmap[mc.iface] = *status
			statusmapMutex.Unlock()
		})
	}
	rg.Wait()

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Debug("retrieved interface status")

	return
}

// Update the configuration for all (or a set of) interfaces
func (cm *Manager) Update(ctx context.Context, ifaces config.Ifaces) (enabled, updated, disabled []string, err error) {
	// Validate the config before doing anything else
	err = ifaces.Validate()
	if err != nil {
		return
	}

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build set of interfaces to enable / disable
	var (
		ifaceSet                                  = make(map[string]struct{})
		enableIfaces, updateIfaces, disableIfaces []string
	)

	cm.Lock()
	for iface, cfg := range ifaces {
		ifaceSet[iface] = struct{}{}
		if _, exists := cm.captures.Get(iface); !exists {
			enableIfaces = append(enableIfaces, iface)
		} else {
			updatedCfg := cfg
			runtimeCfg := cm.lastAppliedConfig[iface]

			// take care of parameter updates to an interface that exists already
			if !updatedCfg.Equals(runtimeCfg) {
				updateIfaces = append(updateIfaces, iface)
			}
		}
	}
	cm.Unlock()

	for iface := range cm.captures.Map {
		if _, exists := ifaceSet[iface]; !exists {
			disableIfaces = append(disableIfaces, iface)
		}
	}

	var disable = append(disableIfaces, updateIfaces...)
	var enable = append(enableIfaces, updateIfaces...)

	cm.update(ctx, ifaces, enable, disable)

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		slog.Group("ifaces",
			"added", enableIfaces,
			"updated", updateIfaces,
			"removed", disableIfaces,
		),
	).Debug("updated interface configuration")

	return enableIfaces, updateIfaces, disableIfaces, nil

}

func (cm *Manager) update(ctx context.Context, ifaces config.Ifaces, enable, disable []string) {

	// execute a final writeout of all disabled interfaces in the list
	if len(disable) > 0 {
		cm.performWriteout(ctx, time.Now().Add(time.Second), disable...)
	}

	// To avoid any interference the update() logic is protected as a whole
	// This also allows us to interace with the captures without copying (creating potential races)
	cm.Lock()
	defer cm.Unlock()

	// store the configuration so that changes can be communicated
	cm.lastAppliedConfig = ifaces

	// Disable any interfaces present in the negative list
	var rg RunGroup
	for _, iface := range disable {
		mc, exists := cm.captures.Get(iface)
		if !exists {
			continue
		}
		rg.Run(func() {

			runCtx := withIfaceContext(ctx, mc.iface)

			logger := logging.FromContext(runCtx)
			logger.Info("closing capture / stopping packet processing")

			if err := mc.close(); err != nil {
				logger.Errorf("failed to close capture: %s", err)
				return
			}

			cm.captures.Delete(mc.iface)
		})
	}
	rg.Wait()

	// Enable any interfaces present in the positive list
	for _, iface := range enable {
		iface := iface

		rg.Run(func() {

			runCtx := withIfaceContext(ctx, iface)
			logger := logging.FromContext(runCtx)

			logger.Info("initializing capture / running packet processing")

			cap := newCapture(iface, ifaces[iface]).SetSourceInitFn(cm.sourceInitFn)
			if err := cap.run(runCtx); err != nil {
				logger.Errorf("failed to start capture: %s", err)
				return
			}

			cm.captures.Set(iface, cap)
		})
	}
	rg.Wait()
}

// Close stops / closes all (or a set of) interfaces
func (cm *Manager) Close(ctx context.Context, ifaces ...string) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	if ifaces = cm.captures.Ifaces(ifaces...); len(ifaces) == 0 {
		return
	}

	// Close all interfaces in the list using update() with the respective list of
	// interfaces to remove
	cm.update(ctx, nil, nil, ifaces)

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Debug("closed interfaces")
}

func withIfaceContext(ctx context.Context, iface string) context.Context {
	return logging.WithFields(ctx, slog.String("iface", iface))
}

func (cm *Manager) rotate(ctx context.Context, writeoutChan chan<- capturetypes.TaggedAggFlowMap, ifaces ...string) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	if ifaces = cm.captures.Ifaces(ifaces...); len(ifaces) == 0 {
		return
	}

	var rg RunGroup
	for _, iface := range ifaces {
		mc, exists := cm.captures.Get(iface)
		if exists {
			rg.Run(func() {

				runCtx := withIfaceContext(ctx, mc.iface)

				// Lock the running capture and perform the rotation
				mc.lock(true)
				mc.unlock()

				rotateResult := mc.rotate(runCtx)

				// Since the capture is locked we can safely extract the (capture) status
				// from the individual interfaces (and unlock no matter what)
				stats, err := mc.status()
				if err != nil {
					logging.FromContext(runCtx).Errorf("failed to get capture stats: %v", err)
				}
				mc.lock(false)
				mc.unlock()

				writeoutChan <- capturetypes.TaggedAggFlowMap{
					Map:   rotateResult,
					Stats: *stats,
					Iface: mc.iface,
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
