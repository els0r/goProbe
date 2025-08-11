package plugins

import (
	"log/slog"
	"sync"
)

func init() {
	GetInitializer()
}

type pluginType string

const (
	querierPlugin  pluginType = "querier"
	resolverPlugin pluginType = "resolver"
)

// Initializer is a singleton that holds all registered plugins
type Initializer struct {
	sync.RWMutex
	queriers  map[string]QuerierInitializer
	resolvers map[string]ResolverInitializer
}

func (i *Initializer) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("queriers", i.getQueriers()),
		slog.Any("resolvers", i.getResolvers()),
	)
}

// GetAvailable Plugins returns a list of all registered plugins by plugin type
func GetAvailablePlugins() map[string][]string {
	return map[string][]string{
		string(querierPlugin):  GetAvailableQuerierPlugins(),
		string(resolverPlugin): GetAvailableResolverPlugins(),
	}
}

var singleton *Initializer
var once sync.Once

// GetInitializer returns the singleton Initializer instance. It is safe to call this function
// concurrently. Repeated calls will return the same instance
func GetInitializer() *Initializer {
	once.Do(func() {
		singleton = &Initializer{
			queriers:  make(map[string]QuerierInitializer),
			resolvers: make(map[string]ResolverInitializer),
		}
	})
	return singleton
}
