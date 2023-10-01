package plugins

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
)

func init() {
	getPluginInitializer()
}

type pluginType string

const (
	querierPlugin pluginType = "querier"
)

type pluginInitializer struct {
	sync.RWMutex
	queriers map[string]QuerierInitializer
}

// QuerierInitializer is a function that initializes a querier instance. The cfgPath parameter
// mandates that the querier plugin must be able to read in its configuration from a file
type QuerierInitializer func(ctx context.Context, cfgPath string) (distributed.Querier, error)

// RegisterQuerier registers a querier initializer function with a given name.
// This function is meant to be used by querier plugins to register themselves.
// RegisterQuerier will panic if a querier with the same name has already been registered
func RegisterQuerier(name string, initFn QuerierInitializer) {
	getPluginInitializer().registerQuerier(name, initFn)
}

// GetAvailablePlugins returns a list of all registered querier plugins
func GetAvailableQuerierPlugins() []string {
	plugins := make([]string, 0)
	for k := range getPluginInitializer().queriers {
		plugins = append(plugins, k)
	}
	sort.StringSlice(plugins).Sort()
	return plugins
}

// InitQuerier will initialize a querier plugin with the given name and configuration path.
// If the plugin never registered itself, an error will be returned
func InitQuerier(ctx context.Context, name, cfgPath string) (distributed.Querier, error) {
	initFn, exists := getPluginInitializer().getQuerier(name)
	if !exists {
		return nil, fmt.Errorf("querier plugin %q not registered", name)
	}
	return initFn(ctx, cfgPath)
}

func (p *pluginInitializer) getQuerier(name string) (QuerierInitializer, bool) {
	p.RLock()
	initFn, exists := p.queriers[name]
	p.RUnlock()

	return initFn, exists
}

func (p *pluginInitializer) registerQuerier(name string, initFn QuerierInitializer) {
	p.Lock()
	_, exists := p.queriers[name]
	if exists {
		panic(fmt.Sprintf("%q querier already registered", name))
	}
	p.queriers[name] = initFn
	p.Unlock()
}

var singleton *pluginInitializer
var once sync.Once

func getPluginInitializer() *pluginInitializer {
	once.Do(func() {
		singleton = &pluginInitializer{
			queriers: make(map[string]QuerierInitializer),
		}
	})
	return singleton
}
