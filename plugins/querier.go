package plugins

import (
	"context"
	"fmt"
	"sort"

	"github.com/els0r/goProbe/v4/pkg/distributed"
)

// QuerierInitializer is a function that initializes a querier instance. The cfgPath parameter
// mandates that the querier plugin must be able to read in its configuration from a file
type QuerierInitializer func(ctx context.Context, cfgPath string) (distributed.Querier, error)

// RegisterQuerier registers a querier initializer function with a given name.
// This function is meant to be used by querier plugins to register themselves.
// RegisterQuerier will panic if a querier with the same name has already been registered
func RegisterQuerier(name string, initFn QuerierInitializer) {
	GetInitializer().registerQuerier(name, initFn)
}

// GetAvailableQuerierPlugins returns a list of all registered querier plugins
func GetAvailableQuerierPlugins() []string {
	return GetInitializer().getQueriers()
}

func (i *Initializer) getQueriers() []string {
	plugins := make([]string, 0)

	i.RLock()
	for k := range i.queriers {
		plugins = append(plugins, k)
	}
	i.RUnlock()

	sort.StringSlice(plugins).Sort()
	return plugins
}

// InitQuerier will initialize a querier plugin with the given name and configuration path.
// If the plugin never registered itself, an error will be returned
func InitQuerier(ctx context.Context, name, cfgPath string) (distributed.Querier, error) {
	initFn, exists := GetInitializer().getQuerier(name)
	if !exists {
		return nil, fmt.Errorf("querier plugin %q not registered", name)
	}
	return initFn(ctx, cfgPath)
}

func (p *Initializer) getQuerier(name string) (QuerierInitializer, bool) {
	p.RLock()
	initFn, exists := p.queriers[name]
	p.RUnlock()

	return initFn, exists
}

func (p *Initializer) registerQuerier(name string, initFn QuerierInitializer) {
	p.Lock()
	_, exists := p.queriers[name]
	if exists {
		panic(fmt.Sprintf("%q querier already registered", name))
	}
	p.queriers[name] = initFn
	p.Unlock()
}
