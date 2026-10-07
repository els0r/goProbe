package plugins

import (
	"context"
	"log/slog"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/stretchr/testify/require"
)

// fakeAuthorizer is a minimal implementation of authz.Authorizer for testing
type fakeAuthorizer struct {
	name string
	cfg  string
}

func (f *fakeAuthorizer) Authorize(_ context.Context, _ authz.Request) (authz.Scope, error) {
	return nil, authz.ErrForbidden
}

// makeFakeAuthorizerInitializer creates an initializer that captures the cfgPath
func makeFakeAuthorizerInitializer(name string) AuthorizerInitializer {
	return func(_ context.Context, cfgPath string) (authz.Authorizer, error) {
		return &fakeAuthorizer{name: name, cfg: cfgPath}, nil
	}
}

// resetAuthorizers clears the registered authorizers between tests to avoid cross-test interference
func resetAuthorizers(tb testing.TB) {
	tb.Helper()
	initr := GetInitializer()
	initr.Lock()
	initr.authorizers = make(map[string]AuthorizerInitializer)
	initr.Unlock()
}

func TestGetAvailablePlugins_ListsAuthorizers(t *testing.T) {
	resetAuthorizers(t)
	RegisterAuthorizer("beta", makeFakeAuthorizerInitializer("beta"))
	RegisterAuthorizer("alpha", makeFakeAuthorizerInitializer("alpha"))

	require.Equal(t, []string{"alpha", "beta"}, GetAvailableAuthorizerPlugins())

	listing := GetAvailablePlugins()
	require.Equal(t, []string{"alpha", "beta"}, listing["authorizer"])
	require.Contains(t, listing, "querier")
	require.Contains(t, listing, "resolver")
}

func TestInitializer_LogValue_ListsAuthorizers(t *testing.T) {
	resetAuthorizers(t)
	RegisterAuthorizer("alpha", makeFakeAuthorizerInitializer("alpha"))

	groups := map[string]slog.Value{}
	for _, attr := range GetInitializer().LogValue().Group() {
		groups[attr.Key] = attr.Value
	}
	require.Contains(t, groups, "queriers")
	require.Contains(t, groups, "resolvers")
	require.Contains(t, groups, "authorizers")
	require.Equal(t, []string{"alpha"}, groups["authorizers"].Any())
}

func TestInitAuthorizer_Success(t *testing.T) {
	resetAuthorizers(t)
	RegisterAuthorizer("foo", makeFakeAuthorizerInitializer("foo"))

	a, err := InitAuthorizer(context.Background(), "foo", "config.yaml")
	require.NoError(t, err)
	require.NotNil(t, a)
	fa, ok := a.(*fakeAuthorizer)
	require.True(t, ok)
	require.Equal(t, "config.yaml", fa.cfg)
}

func TestInitAuthorizer_NotRegistered(t *testing.T) {
	resetAuthorizers(t)
	a, err := InitAuthorizer(context.Background(), "does-not-exist", "")
	require.Error(t, err)
	require.Nil(t, a)
	require.Contains(t, err.Error(), `"does-not-exist"`)
}

func TestRegisterAuthorizer_DuplicatePanics(t *testing.T) {
	resetAuthorizers(t)
	RegisterAuthorizer("dup", makeFakeAuthorizerInitializer("dup"))
	require.Panics(t, func() {
		RegisterAuthorizer("dup", makeFakeAuthorizerInitializer("dup2"))
	})
}
