package config

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/els0r/goProbe/pkg/logging"
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
type CallbackFn func(context.Context, Ifaces) ([]string, []string, []string, error)

// MonitorOption denotes a functional option for a config monitor
type MonitorOption func(*Monitor)

// WithReloadInterval sets a non-default config reload interval
func WithReloadInterval(interval time.Duration) MonitorOption {
	return func(m *Monitor) {
		m.reloadInterval = interval
	}
}

// NewMonitor instantiates a new config monitor and performs an initial read of the
// provided config file
func NewMonitor(path string, opts ...MonitorOption) (*Monitor, error) {
	config, err := ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

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

// Reload triggers a config reload (and executes the callback function, if provided)
func (m *Monitor) Reload(ctx context.Context, fn CallbackFn) error {
	config, err := ParseFile(m.path)
	if err != nil {
		return fmt.Errorf("failed to reload config file: %w", err)
	}

	if fn != nil {
		if _, _, _, err = fn(ctx, config.Interfaces); err != nil {
			return fmt.Errorf("failed to execute config reload callback function: %w", err)
		}
	}

	m.Lock()
	m.config = config
	m.Unlock()

	logging.FromContext(ctx).Debugf("config reloaded from %s", m.path)

	return nil
}

////////////////////////////////////////////////////////////////////////

func (m *Monitor) reloadPeriodically(ctx context.Context, fn CallbackFn) {

	logger := logging.FromContext(ctx)
	ticker := time.NewTicker(m.reloadInterval)
	logger.Infof("starting config monitor (interval: %v)", m.reloadInterval)

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping config monitor")
			ticker.Stop()
			return
		case <-ticker.C:
			if err := m.Reload(ctx, fn); err != nil {
				logger.Errorf("failed to perform periodic config reload: %s", err)
			}
		}
	}
}
