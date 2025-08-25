package plugins

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/telemetry/logging"
)

// ResolverConfig is the configuration relevant for resolver plugin configuration
type ResolverConfig struct {
	Type   string `mapstructure:"type"`   // Type is the type of the resolver (e.g. the name)
	Config string `mapstructure:"config"` // Config is the path to the configuration file
}

// HostResolverConfig holds all resolver configuration
type HostResolverConfig struct {
	Resolvers []*ResolverConfig `mapstructure:"resolvers"`
}

// AppConfig holds the application configuration configurable through viper only
type AppConfig struct {
	Hosts *HostResolverConfig `mapstructure:"hosts"`
}

// ResolverInitializer constructs a resolver, optionally using a config file.
// Mirrors the existing QuerierInitializer pattern
type ResolverInitializer func(ctx context.Context, cfgPath string) (hosts.Resolver, error)

// RegisterResolver is typically called from init() in a resolver package.
func RegisterResolver(name string, initFn ResolverInitializer) {
	GetInitializer().registerResolver(name, initFn)
}

// GetAvailableResolverPlugins returns a list of all registered resolver plugins
func GetAvailableResolverPlugins() []string {
	return GetInitializer().getResolvers()
}

func (i *Initializer) getResolvers() []string {
	i.RLock()
	plugins := make([]string, 0, len(i.resolvers))

	for k := range i.resolvers {
		plugins = append(plugins, k)
	}
	i.RUnlock()

	sort.StringSlice(plugins).Sort()
	return plugins
}

// InitResolver returns a resolver by name or an error if it isn't registered.
func InitResolver(ctx context.Context, name, cfgPath string) (hosts.Resolver, error) {
	initFn, exists := GetInitializer().getResolver(name)
	if !exists {
		return nil, fmt.Errorf("resolver plugin %q not registered", name)
	}
	return initFn(ctx, cfgPath)
}

// InitResolvers initializes all registered resolver plugins
func InitResolvers(ctx context.Context, cfg *HostResolverConfig) (*hosts.ResolverMap, error) {
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return nil, errors.New("host resolver config is nil")
	}

	var rm = hosts.NewResolverMap()

	for _, resolverCfg := range cfg.Resolvers {
		if resolverCfg == nil {
			logger.Warn("nil resolver config found")
			continue
		}
		logger.WithGroup("resolver").With("type", resolverCfg.Type, "config", resolverCfg.Config).Info("initializing resolver")

		name := resolverCfg.Type
		initFn, exists := GetInitializer().getResolver(name)
		if !exists {
			return nil, fmt.Errorf("resolver plugin %q not registered", name)
		}
		resolver, err := initFn(ctx, resolverCfg.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize resolver %q: %w", name, err)
		}
		rm.Set(name, resolver)
	}
	return rm, nil
}

// getResolver returns the resolver for a given name in case it exists
func (i *Initializer) getResolver(name string) (ResolverInitializer, bool) {
	i.RLock()
	initFn, exists := i.resolvers[name]
	i.RUnlock()

	return initFn, exists
}

func (i *Initializer) registerResolver(name string, initFn ResolverInitializer) {
	i.Lock()
	if _, exists := i.resolvers[name]; exists {
		panic(fmt.Sprintf("%q resolver already registered", name))
	}
	i.resolvers[name] = initFn
	i.Unlock()
}
