package cmd

import (
	"context"

	"github.com/spf13/viper"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/v4/pkg/distributed"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins"

	// internal plugin support
	_ "github.com/els0r/goProbe/v4/plugins/querier"
	_ "github.com/els0r/goProbe/v4/plugins/resolver"
)

func initQuerier(ctx context.Context) (querier distributed.Querier, err error) {
	return plugins.InitQuerier(ctx,
		viper.GetString(conf.QuerierType),
		viper.GetString(conf.QuerierConfig),
	)
}

func initResolver(ctx context.Context) (resolver hosts.Resolver, err error) {
	return plugins.InitResolver(ctx,
		viper.GetString(conf.HostsResolverType),
		viper.GetString(conf.HostsResolverConfig),
	)
}
