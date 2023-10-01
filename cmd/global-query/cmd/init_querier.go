package cmd

import (
	"context"

	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/plugins"
	"github.com/spf13/viper"
)

func initQuerier(ctx context.Context) (querier distributed.Querier, err error) {
	return plugins.InitQuerier(ctx,
		viper.GetString(conf.QuerierType),
		viper.GetString(conf.QuerierConfig),
	)
}
