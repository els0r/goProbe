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
	querierPlugin    pluginType = "querier"
	resolverPlugin   pluginType = "resolver"
	authorizerPlugin pluginType = "authorizer"
)

// Initializer is a singleton that holds all registered plugins
type Initializer struct {
	sync.RWMutex
	queriers    map[string]QuerierInitializer
	resolvers   map[string]ResolverInitializer
	authorizers map[string]AuthorizerInitializer
}

func (i *Initializer) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("queriers", i.getQueriers()),
		slog.Any("resolvers", i.getResolvers()),
		slog.Any("authorizers", i.getAuthorizers()),
	)
}

// GetAvailablePlugins returns a list of all registered plugins by plugin type
func GetAvailablePlugins() map[string][]string {
	return map[string][]string{
		string(querierPlugin):    GetAvailableQuerierPlugins(),
		string(resolverPlugin):   GetAvailableResolverPlugins(),
		string(authorizerPlugin): GetAvailableAuthorizerPlugins(),
	}
}

var singleton *Initializer
var once sync.Once

// GetInitializer returns the singleton Initializer instance. It is safe to call this function
// concurrently. Repeated calls will return the same instance
func GetInitializer() *Initializer {
	once.Do(func() {
		singleton = &Initializer{
			queriers:    make(map[string]QuerierInitializer),
			resolvers:   make(map[string]ResolverInitializer),
			authorizers: make(map[string]AuthorizerInitializer),
		}
	})
	return singleton
}
