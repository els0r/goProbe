package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
	"github.com/els0r/telemetry/logging"
)

const defaultReloadInterval = 5 * time.Minute

// Monitor denotes a config monitor / manager
type Monitor struct {
	path   string
	config *Config

	reloadInterval time.Duration

	sync.RWMutex
}

// CallbackFn denotes a function to be called upon successful reload of the configuration
type CallbackFn func(context.Context, Ifaces) (enabled, updated, disabled capturetypes.IfaceChanges, err error)

// MonitorOption denotes a functional option for a config monitor
type MonitorOption func(*Monitor)

// WithReloadInterval sets a non-default config reload interval
func WithReloadInterval(interval time.Duration) MonitorOption {
	return func(m *Monitor) {
		m.reloadInterval = interval
	}
}

// WithInitialConfig sets an initial configuration for the monitor. It will override the one read from `path` in NewMonitor
func WithInitialConfig(cfg *Config) MonitorOption {
	return func(m *Monitor) {
		m.config = cfg
	}
}

// NewMonitor instantiates a new config monitor and performs an initial read of the
// provided config file
func NewMonitor(path string, opts ...MonitorOption) (*Monitor, error) {
	var (
		config *Config
	)

	obj := &Monitor{
		path:           path,
		config:         config,
		reloadInterval: defaultReloadInterval,
	}

	// Execute functional options, if any
	for _, opt := range opts {
		opt(obj)
	}

	return obj, nil
}

// GetConfig safely returns the current configuration
func (m *Monitor) GetConfig() (cfg *Config) {
	m.RLock()
	cfg = m.config
	m.RUnlock()

	return
}

// PutConfig safely updates the configuration with a new one
func (m *Monitor) PutConfig(cfg *Config) {
	m.Lock()
	m.config = cfg
	m.Unlock()
}

// PutIfaceConfig safely updates the interface configuration with a new one
func (m *Monitor) PutIfaceConfig(cfg Ifaces) {
	m.Lock()
	m.config.Interfaces = cfg
	m.Unlock()
}

// Start initializaes the config monitor background task(s)
func (m *Monitor) Start(ctx context.Context, fn CallbackFn) {
	go m.reloadPeriodically(ctx, fn)
}

// Reload triggers a config reload from disk and triggers the execution of the provided callback (if any)
func (m *Monitor) Reload(ctx context.Context, fn CallbackFn) (enabled, updated, disabled capturetypes.IfaceChanges, err error) {
	if m.path == "" {
		return
	}

	cfg, perr := ParseFile(m.path)
	if perr != nil {
		err = fmt.Errorf("failed to reload config file: %w", perr)
		return
	}
	if err = cfg.Validate(); err != nil {
		err = fmt.Errorf("reloaded config file is invalid: %w", err)
		return
	}

	m.PutConfig(cfg)

	logging.FromContext(ctx).With("path", m.path).Debugf("config reloaded")

	if fn != nil {
		return m.Apply(ctx, fn)
	}

	return
}

// Apply peforms a callback to the provided function and returns its result
func (m *Monitor) Apply(ctx context.Context, fn CallbackFn) (enabled, updated, disabled capturetypes.IfaceChanges, err error) {

	if fn == nil {
		err = fmt.Errorf("no callback function provided")
		return
	}

	if enabled, updated, disabled, err = fn(ctx, m.config.Interfaces); err != nil {
		err = fmt.Errorf("failed to execute config reload callback function: %w", err)
		return
	}

	logging.FromContext(ctx).With(
		"enabled", enabled,
		"updated", updated,
		"disabled", disabled,
	).Debug("config applied")

	return
}

////////////////////////////////////////////////////////////////////////

func (m *Monitor) reloadPeriodically(ctx context.Context, fn CallbackFn) {

	logger := logging.FromContext(ctx)
	ticker := time.NewTicker(m.reloadInterval)
	logger.With("interval", m.reloadInterval.Round(1*time.Second)).Info("starting config monitor")

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping config monitor")
			ticker.Stop()
			return
		case <-ticker.C:
			if _, _, _, err := m.Reload(ctx, fn); err != nil {
				logger.Errorf("failed to perform periodic config reload: %s", err)
			}
		}
	}
}
