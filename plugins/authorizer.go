package plugins

import (
	"context"
	"fmt"
	"sort"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
)

// AuthorizerInitializer constructs an authorizer, optionally using a config file.
// Mirrors the QuerierInitializer and ResolverInitializer pattern
type AuthorizerInitializer func(ctx context.Context, cfgPath string) (authz.Authorizer, error)

// RegisterAuthorizer registers an authorizer initializer function with a given name.
// It is typically called from init() in an authorizer package.
// RegisterAuthorizer will panic if an authorizer with the same name has already been registered
func RegisterAuthorizer(name string, initFn AuthorizerInitializer) {
	GetInitializer().registerAuthorizer(name, initFn)
}

// GetAvailableAuthorizerPlugins returns a list of all registered authorizer plugins
func GetAvailableAuthorizerPlugins() []string {
	return GetInitializer().getAuthorizers()
}

func (i *Initializer) getAuthorizers() []string {
	i.RLock()
	plugins := make([]string, 0, len(i.authorizers))

	for k := range i.authorizers {
		plugins = append(plugins, k)
	}
	i.RUnlock()

	sort.StringSlice(plugins).Sort()
	return plugins
}

// InitAuthorizer initializes the authorizer plugin with the given name and configuration path.
// If the plugin never registered itself, an error naming it is returned
func InitAuthorizer(ctx context.Context, name, cfgPath string) (authz.Authorizer, error) {
	initFn, exists := GetInitializer().getAuthorizer(name)
	if !exists {
		return nil, fmt.Errorf("authorizer plugin %q not registered", name)
	}
	return initFn(ctx, cfgPath)
}

// getAuthorizer returns the authorizer initializer for a given name in case it exists
func (i *Initializer) getAuthorizer(name string) (AuthorizerInitializer, bool) {
	i.RLock()
	initFn, exists := i.authorizers[name]
	i.RUnlock()

	return initFn, exists
}

func (i *Initializer) registerAuthorizer(name string, initFn AuthorizerInitializer) {
	i.Lock()
	defer i.Unlock()
	if _, exists := i.authorizers[name]; exists {
		panic(fmt.Sprintf("%q authorizer already registered", name))
	}
	i.authorizers[name] = initFn
}
