package plugins

import (
	"context"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/stretchr/testify/require"
)

// fakeResolver is a minimal implementation of hosts.Resolver for testing
type fakeResolver struct {
	name string
	cfg  string
}

func (f *fakeResolver) Resolve(_ context.Context, query string) (hosts.Hosts, error) {
	return hosts.Hosts{f.name + ":" + query}, nil
}

// helper to create an initializer that captures the cfgPath
func makeFakeInitializer(name string) ResolverInitializer {
	return func(_ context.Context, cfgPath string) (hosts.Resolver, error) {
		return &fakeResolver{name: name, cfg: cfgPath}, nil
	}
}

// resetResolvers clears the registered resolvers between tests to avoid cross-test interference
func resetResolvers(tb testing.TB) {
	tb.Helper()
	initr := GetInitializer()
	initr.Lock()
	initr.resolvers = make(map[string]ResolverInitializer)
	initr.Unlock()
}

func TestGetAvailableResolverPlugins_Sorted(t *testing.T) {
	resetResolvers(t)
	RegisterResolver("beta", makeFakeInitializer("beta"))
	RegisterResolver("alpha", makeFakeInitializer("alpha"))

	got := GetAvailableResolverPlugins()
	want := []string{"alpha", "beta"}
	require.Equal(t, want, got)
}

func TestInitResolver_Success(t *testing.T) {
	resetResolvers(t)

	RegisterResolver("foo", func(_ context.Context, cfgPath string) (hosts.Resolver, error) {
		return &fakeResolver{name: "foo", cfg: cfgPath}, nil
	})

	r, err := InitResolver(context.Background(), "foo", "config.yaml")
	require.NoError(t, err)
	require.NotNil(t, r)
	fr, ok := r.(*fakeResolver)
	require.True(t, ok)
	require.Equal(t, "config.yaml", fr.cfg)
}

func TestInitResolver_NotRegistered(t *testing.T) {
	resetResolvers(t)
	_, err := InitResolver(context.Background(), "does-not-exist", "")
	require.Error(t, err)
}

func TestInitResolvers_NilConfig(t *testing.T) {
	resetResolvers(t)
	_, err := InitResolvers(context.Background(), nil)
	require.Error(t, err)
}

func TestInitResolvers_UnknownPlugin(t *testing.T) {
	resetResolvers(t)
	cfg := &HostResolverConfig{Resolvers: []*ResolverConfig{{Type: "unknown", Config: "cfg.yaml"}}}
	_, err := InitResolvers(context.Background(), cfg)
	require.Error(t, err)
}

func TestInitResolvers_Success_WithNilEntry(t *testing.T) {
	resetResolvers(t)
	RegisterResolver("alpha", makeFakeInitializer("alpha"))
	RegisterResolver("beta", makeFakeInitializer("beta"))

	cfg := &HostResolverConfig{Resolvers: []*ResolverConfig{
		nil, // should be skipped
		{Type: "alpha", Config: "a.yaml"},
		{Type: "beta", Config: "b.yaml"},
	}}

	rm, err := InitResolvers(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, rm)

	r, ok := rm.Get("alpha")
	require.True(t, ok, "expected resolver 'alpha' to be present")
	require.NotNil(t, r)
	require.Equal(t, "a.yaml", r.(*fakeResolver).cfg)

	r, ok = rm.Get("beta")
	require.True(t, ok, "expected resolver 'beta' to be present")
	require.NotNil(t, r)
	require.Equal(t, "b.yaml", r.(*fakeResolver).cfg)
}

func TestRegisterResolver_DuplicatePanics(t *testing.T) {
	resetResolvers(t)
	RegisterResolver("dup", makeFakeInitializer("dup"))
	require.Panics(t, func() {
		RegisterResolver("dup", makeFakeInitializer("dup2"))
	})
}
