package capture

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/capture/capturetypes"
	"github.com/els0r/goProbe/pkg/goDB"
	"github.com/els0r/goProbe/pkg/goDB/encoder/encoders"
	"github.com/els0r/goProbe/pkg/goprobe/writeout"
	"github.com/els0r/goProbe/pkg/types/hashmap"
	"github.com/els0r/telemetry/logging"
	"github.com/fako1024/gotools/concurrency"
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

	skipWriteoutSchedule bool

	localBufferPool *LocalBufferPool
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

	// Initialize local buffer
	if err := captureManager.setLocalBuffers(); err != nil {
		return nil, fmt.Errorf("failed to set local buffer(s): %w", err)
	}

	// Update (i.e. start) all capture routines (implicitly by reloading all configurations) and schedule
	// DB writeouts
	_, _, _, err = captureManager.Update(ctx, config.Interfaces)
	if err != nil {
		return nil, err
	}

	// this is the first time the capture manager is started and is important to report program runtime
	captureManager.startedAt = time.Now()

	if !captureManager.skipWriteoutSchedule {
		captureManager.ScheduleWriteouts(ctx, time.Duration(goDB.DBWriteInterval)*time.Second)
	}

	return captureManager, nil
}

// NewManager creates a new CaptureManager
func NewManager(writeoutHandler writeout.Handler, opts ...ManagerOption) *Manager {
	captureManager := &Manager{
		captures:        newCaptures(),
		writeoutHandler: writeoutHandler,
		sourceInitFn:    defaultSourceInitFn,

		// This is explicit here to ensure that each manager by default has its own memory pool (unless injected)
		localBufferPool: NewLocalBufferPool(1, config.DefaultLocalBufferSizeLimit),
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
				ticker.Stop()
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

// WithSkipWriteoutSchedule disables scheduled writeouts
func WithSkipWriteoutSchedule(skip bool) ManagerOption {
	return func(cm *Manager) {
		cm.skipWriteoutSchedule = skip
	}
}

// WithLocalBuffers sets one or multiple local buffers for the capture manager
func WithLocalBuffers(nBuffers, sizeLimit int) ManagerOption {
	return func(cm *Manager) {
		cm.localBufferPool.NBuffers = nBuffers
		cm.localBufferPool.MaxBufferSize = sizeLimit
	}
}

// Config returns the runtime config of the capture manager for all (or a set of) interfaces
func (cm *Manager) Config(ifaces ...string) (ifaceConfigs config.Ifaces) {
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

	cm.Lock()
	defer cm.Unlock()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	if ifaces = cm.captures.Ifaces(ifaces...); len(ifaces) == 0 {
		return
	}

	for _, iface := range ifaces {
		mc, exists := cm.captures.Get(iface)
		if !exists {
			continue
		}

		runCtx := withIfaceContext(ctx, mc.iface)

		// Lock the running capture
		if err := mc.capLock.Lock(); err != nil {
			logger := logging.FromContext(runCtx)
			logger.Errorf("failed to establish status call three-point lock: %s", err)
			if err := mc.close(); err != nil {
				logger.Errorf("failed to close capture after failed three-point lock: %s", err)
			}
			cm.captures.Delete(mc.iface)

			return
		}

		// Since the capture is locked we can safely extract the (capture) status
		// from the individual interfaces (and unlock no matter what)
		status, err := mc.status()
		if lErr := mc.capLock.Unlock(); lErr != nil {
			logger := logging.FromContext(runCtx)
			logger.Errorf("failed to release status call three-point lock: %s", err)
			if err := mc.close(); err != nil {
				logger.Errorf("failed to close capture after failed three-point lock: %s", err)
			}
			cm.captures.Delete(mc.iface)
		}

		if err != nil {
			logging.FromContext(runCtx).Errorf("failed to get capture stats: %s", err)
			return
		}

		statusmap[mc.iface] = *status
	}

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Debug("retrieved interface status")

	return
}

// Update the configuration for all (or a set of) interfaces
func (cm *Manager) Update(ctx context.Context, ifaces config.Ifaces) (enabled, updated, disabled capturetypes.IfaceChanges, err error) {
	// Validate the config before doing anything else
	err = ifaces.Validate()
	if err != nil {
		return
	}

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build set of interfaces to enable / disable
	var (
		ifaceSet                                  = make(map[string]struct{})
		enableIfaces, updateIfaces, disableIfaces capturetypes.IfaceChanges
	)

	cm.Lock()
	for iface, cfg := range ifaces {
		ifaceSet[iface] = struct{}{}
		if _, exists := cm.captures.Get(iface); !exists {
			enableIfaces = append(enableIfaces, capturetypes.IfaceChange{Name: iface})
		} else {
			updatedCfg := cfg
			runtimeCfg := cm.lastAppliedConfig[iface]

			// take care of parameter updates to an interface that exists already
			if !updatedCfg.Equals(runtimeCfg) {
				updateIfaces = append(updateIfaces, capturetypes.IfaceChange{Name: iface})
			}
		}
	}
	cm.Unlock()

	for _, iface := range cm.captures.Ifaces() {
		if _, exists := ifaceSet[iface]; !exists {
			disableIfaces = append(disableIfaces, capturetypes.IfaceChange{Name: iface})
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
	).Info("updated interface configuration")

	return enableIfaces, updateIfaces, disableIfaces, nil

}

func (cm *Manager) update(ctx context.Context, ifaces config.Ifaces, enable, disable capturetypes.IfaceChanges) {

	// execute a final writeout of all disabled interfaces in the list
	if len(disable) > 0 {
		cm.performWriteout(ctx, time.Now().Add(time.Second), disable.Names()...)
	}

	// To avoid any interference the update() logic is protected as a whole
	// This also allows us to interface with the captures without copying (creating potential races)
	cm.Lock()
	defer cm.Unlock()

	// store the configuration so that changes can be communicated
	cm.lastAppliedConfig = ifaces

	// Disable any interfaces present in the negative list
	var rg RunGroup
	for i, iface := range disable {
		iface := iface

		mc, exists := cm.captures.Get(iface.Name)
		if !exists {
			continue
		}
		rg.Run(func() {

			runCtx := withIfaceContext(ctx, mc.iface)

			logger := logging.FromContext(runCtx)
			logger.Info("closing capture / stopping packet processing")
			if err := mc.close(); err != nil {
				logger.Errorf("failed to close capture during config update: %s", err)
			} else {
				disable[i].Success = true
			}

			cm.captures.Delete(mc.iface)
		})
	}
	rg.Wait()

	// Enable any interfaces present in the positive list
	for i, iface := range enable {
		iface := iface

		rg.Run(func() {

			runCtx := withIfaceContext(ctx, iface.Name)
			logger := logging.FromContext(runCtx)

			logger.Info("initializing capture / running packet processing")

			newCap := newCapture(iface.Name, ifaces[iface.Name]).SetSourceInitFn(cm.sourceInitFn)
			if err := newCap.run(cm.localBufferPool); err != nil {
				logger.Errorf("failed to start capture: %s", err)
				return
			}
			enable[i].Success = true

			// Start up processing and error handling / logging in the
			// background
			errChan := newCap.process()
			go cm.logErrors(runCtx, iface.Name, errChan)

			cm.captures.Set(iface.Name, newCap)
		})
	}
	rg.Wait()
}

// GetFlowMaps extracts a copy of all active flows and sends them on the provided channel (compatible with normal query
// processing). This way, live data can be added to a query result
func (cm *Manager) GetFlowMaps(ctx context.Context, filterFn goDB.FilterFn, writeoutChan chan<- hashmap.AggFlowMapWithMetadata, ifaces ...string) {

	logger, t0 := logging.FromContext(ctx), time.Now()

	// Build list of interfaces to process (either from all interfaces or from explicit list)
	// If none are provided / are available, return empty map
	if ifaces = cm.captures.Ifaces(ifaces...); len(ifaces) == 0 {
		return
	}

	for _, iface := range ifaces {
		mc, exists := cm.captures.Get(iface)
		if exists {

			runCtx := withIfaceContext(ctx, mc.iface)

			// Lock the running capture and perform the rotation
			if err := mc.capLock.Lock(); err != nil {
				logger := logging.FromContext(runCtx)
				logger.Errorf("failed to establish GetFlowMaps three-point lock: %s", err)
				if err := mc.close(); err != nil {
					logger.Errorf("failed to close capture after failed three-point lock: %s", err)
				}
				cm.captures.Delete(mc.iface)
				continue
			}
			flowMap := mc.flowMap(runCtx)
			if err := mc.capLock.Unlock(); err != nil {
				logger := logging.FromContext(runCtx)
				logger.Errorf("failed to release GetFlowMaps three-point lock: %s", err)
				if err := mc.close(); err != nil {
					logger.Errorf("failed to close capture after failed three-point lock: %s", err)
				}
				cm.captures.Delete(mc.iface)
			}

			if flowMap != nil {
				if filterFn != nil {
					flowMap = filterFn(flowMap)
				}
				writeoutChan <- hashmap.AggFlowMapWithMetadata{
					AggFlowMap: flowMap,
					Interface:  iface,
				}
			}
		}
	}

	// log fetch duration
	logger.With(
		"elapsed", time.Since(t0).Round(time.Microsecond).String(),
		"ifaces", ifaces,
	).Debug("fetched flow maps")
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
	cm.update(ctx, nil, nil, capturetypes.FromIfaceNames(ifaces))

	logger.With(
		"elapsed", time.Since(t0).Round(time.Millisecond).String(),
		"ifaces", ifaces,
	).Info("closed interfaces")
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

	// Iteratively rotate all interfaces. Since the rotation results are put on the writeoutChan for
	// writeout by the DBWriter (which is sequential and certainly slower than the actual in-memory rotation)
	// there is no significant benefit from running the rotations in parallel, thus allowing us to minimize
	// congestion _and_ use a single shared local memory buffer
	for _, iface := range ifaces {
		if mc, exists := cm.captures.Get(iface); exists {

			runCtx := withIfaceContext(ctx, mc.iface)
			logger, lockStart := logging.FromContext(runCtx), time.Now()

			// Lock the running capture in order to safely perform rotation tasks
			if err := mc.capLock.Lock(); err != nil {
				logger.Errorf("failed to establish rotation three-point lock: %s", err)
				continue
			}

			// Extract capture stats in a separate goroutine to minimize rotation duration
			statsRes := mc.fetchStatusInBackground(runCtx)

			// Perform the rotation
			rotateResult := mc.rotate(runCtx)

			stats := <-statsRes
			if err := mc.capLock.Unlock(); err != nil {
				logger.Errorf("failed to release rotation three-point lock: %s", err)
			}
			logger.With("elapsed", time.Since(lockStart).Round(time.Microsecond).String()).Debug("interface lock-cycle complete")

			writeoutChan <- capturetypes.TaggedAggFlowMap{
				Map:   rotateResult,
				Stats: *stats,
				Iface: mc.iface,
			}
		}
	}

	// observe rotation duration
	t1 := time.Since(t0)
	promRotationDuration.Observe(float64(t1) / float64(time.Second))
	promInterfacesCapturing.Set(float64(len(ifaces)))

	logger.With(
		"elapsed", t1.Round(time.Microsecond).String(),
		"ifaces", ifaces,
	).Info("rotated interfaces")
}

func (cm *Manager) logErrors(ctx context.Context, iface string, errsChan <-chan error) {
	logger := logging.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errsChan:
			if !ok {

				// Ensure there is no conflict with calls to update() that might already be
				// taking down this interface
				cm.Lock()
				defer cm.Unlock()

				// If the error channel was closed prematurely, we have to assume there was
				// a critical processing error and tear down the interface
				if mc, exists := cm.captures.Get(iface); exists {
					logger.Info("closing capture / stopping packet processing")
					if err := mc.close(); err != nil {
						logger.Warnf("failed to close capture in logging routine (might be expected): %s", err)
					}
					cm.captures.Delete(mc.iface)
				}
				return
			}
			logger.Error(err)
		}
	}
}

func (cm *Manager) performWriteout(ctx context.Context, timestamp time.Time, ifaces ...string) {
	writeoutChan := make(chan capturetypes.TaggedAggFlowMap, writeout.WriteoutsChanDepth)
	doneChan := cm.writeoutHandler.HandleWriteout(ctx, timestamp, writeoutChan)

	cm.Lock()
	cm.rotate(ctx, writeoutChan, ifaces...)

	close(writeoutChan)
	<-doneChan

	cm.lastRotation = timestamp
	cm.Unlock()
}

func (cm *Manager) setLocalBuffers() error {

	// Guard against invalid (i.e. zero) buffer size / limits
	if cm.localBufferPool.NBuffers == 0 || cm.localBufferPool.MaxBufferSize == 0 {
		return fmt.Errorf("invalid number of local buffers (%d) / size limit (%d) specified", cm.localBufferPool.NBuffers, cm.localBufferPool.MaxBufferSize)
	}

	if cm.localBufferPool != nil {
		cm.localBufferPool.Clear()
	}
	cm.localBufferPool.MemPoolLimitUnique = concurrency.NewMemPoolLimitUnique(cm.localBufferPool.NBuffers, initialBufferSize)

	return nil
}
