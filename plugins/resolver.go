package plugins

import (
	"context"
	"fmt"
	"sort"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
)

// ResolverInitializer constructs a resolver, optionally using a config file.
// Mirrors the existing QuerierInitializer pattern. :contentReference[oaicite:1]{index=1}
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
	plugins := make([]string, 0)

	i.RLock()
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
