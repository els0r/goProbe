package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins/querier/apiclient"
	"github.com/els0r/goProbe/v4/plugins/resolver/stringresolver"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestInitQuerier_UsesViperAndReturnsAPIClientQuerier(t *testing.T) {
	viper.Reset()

	// Prepare minimal valid API client config
	f, err := os.CreateTemp(t.TempDir(), "api-client-*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	_, err = f.WriteString("host1:\n  addr: localhost:8145\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	viper.Set(conf.QuerierType, apiclient.Name)
	viper.Set(conf.QuerierConfig, f.Name())

	q, err := initQuerier(context.Background())
	require.NoError(t, err)
	require.NotNil(t, q)
	// confirm the concrete type
	_, ok := q.(*apiclient.APIClientQuerier)
	require.True(t, ok, "querier should be *APIClientQuerier")
}

func TestInitResolvers_AppendsFlagAndStringResolver(t *testing.T) {
	viper.Reset()

	// Ensure hosts struct exists on unmarshal; start with empty list
	viper.Set("hosts.resolvers", []map[string]string{})

	// Provide resolver via flags (type=string, config empty) and expect string resolver present
	viper.Set(conf.HostsResolverType, stringresolver.Type)
	viper.Set(conf.HostsResolverConfig, "")

	rm, err := initResolvers(context.Background())
	require.NoError(t, err)
	require.NotNil(t, rm)

	// Should have at least the string resolver registered
	r, ok := rm.Get(stringresolver.Type)
	require.True(t, ok)
	require.Implements(t, (*hosts.Resolver)(nil), r)
}

func TestInitQuerier_UnknownType_ReturnsError(t *testing.T) {
	viper.Reset()
	viper.Set(conf.QuerierType, "does-not-exist")
	viper.Set(conf.QuerierConfig, "")

	q, err := initQuerier(context.Background())
	require.Error(t, err)
	require.Nil(t, q)
}
