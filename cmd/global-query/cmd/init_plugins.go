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
	"github.com/els0r/goProbe/v4/plugins/resolver/stringresolver"
)

func initQuerier(ctx context.Context) (querier distributed.Querier, err error) {
	return plugins.InitQuerier(ctx,
		viper.GetString(conf.QuerierType),
		viper.GetString(conf.QuerierConfig),
	)
}

func initResolvers(ctx context.Context) (resolvers *hosts.ResolverMap, err error) {
	appCfg := &plugins.AppConfig{}
	if err := viper.Unmarshal(&appCfg); err != nil {
		return nil, err
	}

	appCfg.Hosts.Resolvers = append(appCfg.Hosts.Resolvers,
		// supports the flag values
		&plugins.ResolverConfig{
			Type:   viper.GetString(conf.HostsResolverType),
			Config: viper.GetString(conf.HostsResolverConfig),
		},
		// always register the string resolver
		&plugins.ResolverConfig{
			Type: stringresolver.Type,
		})

	return plugins.InitResolvers(ctx, appCfg.Hosts)
}
