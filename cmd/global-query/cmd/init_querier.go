//go:build !contrib

package cmd

import (
	"fmt"

	"github.com/els0r/goProbe/cmd/global-query/pkg/conf"
	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed"
	"github.com/els0r/goProbe/cmd/global-query/pkg/distributed/contrib/querier/apiclient"
	"github.com/spf13/viper"
)

func initQuerier() (distributed.Querier, error) {
	querierType := viper.GetString(conf.QuerierType)
	switch querierType {
	case string(apiclient.Name):
		querier, err := apiclient.New(
			viper.GetString(conf.QuerierConfig),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate API client querier: %w", err)
		}
		return querier.SetMaxConcurrent(viper.GetInt(conf.QuerierMaxConcurrent)), nil
	default:
		err := fmt.Errorf("querier type %q not supported", querierType)
		return nil, err
	}
}
