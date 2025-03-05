package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/distributed"
)

func init() {
	GetInitializer()
}

type pluginType string

const (
	querierPlugin pluginType = "querier"
)

// Initializer is a singleton that holds all registered plugins
type Initializer struct {
	sync.RWMutex
	queriers map[string]QuerierInitializer
}

func (i *Initializer) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("queriers", i.getQueriers()),
	)
}

// QuerierInitializer is a function that initializes a querier instance. The cfgPath parameter
// mandates that the querier plugin must be able to read in its configuration from a file
type QuerierInitializer func(ctx context.Context, cfgPath string) (distributed.Querier, error)

// RegisterQuerier registers a querier initializer function with a given name.
// This function is meant to be used by querier plugins to register themselves.
// RegisterQuerier will panic if a querier with the same name has already been registered
func RegisterQuerier(name string, initFn QuerierInitializer) {
	GetInitializer().registerQuerier(name, initFn)
}

// GetAvailablePlugins returns a list of all registered querier plugins
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

// GetAvailable Plugins returns a list of all registered plugins by plugin type
func GetAvailablePlugins() map[string][]string {
	return map[string][]string{
		string(querierPlugin): GetAvailableQuerierPlugins(),
	}
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

var singleton *Initializer
var once sync.Once

// GetInitializer returns the singleton Initializer instance. It is safe to call this function
// concurrently. Repeated calls will return the same instance
func GetInitializer() *Initializer {
	once.Do(func() {
		singleton = &Initializer{
			queriers: make(map[string]QuerierInitializer),
		}
	})
	return singleton
}
