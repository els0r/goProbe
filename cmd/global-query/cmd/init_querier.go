package cmd

import (
	"context"

	"github.com/spf13/viper"

	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/v4/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/v4/plugins"

	// internal plugin support
	_ "github.com/els0r/goProbe/v4/plugins/querier"
)

func initQuerier(ctx context.Context) (querier distributed.Querier, err error) {
	return plugins.InitQuerier(ctx,
		viper.GetString(conf.QuerierType),
		viper.GetString(conf.QuerierConfig),
	)
}
