package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/els0r/goProbe/v4/cmd/goQuery/pkg/conf"
	"github.com/spf13/viper"
)

// queryServerTokenEnv is the environment variable the query server token is read from
const queryServerTokenEnv = "QUERY_SERVER_TOKEN"

// queryServerToken returns the Bearer token presented to the query server, taken from the
// token file or the token key (config file or environment). An empty token denotes that no
// credential is presented
func queryServerToken() (string, error) {
	// the token has no flag, so that it never shows in the process list
	if err := viper.BindEnv(conf.QueryServerToken, queryServerTokenEnv); err != nil {
		return "", fmt.Errorf("failed to bind %s: %w", queryServerTokenEnv, err)
	}

	token := viper.GetString(conf.QueryServerToken)
	tokenFile := viper.GetString(conf.QueryServerTokenFile)
	if tokenFile == "" {
		return token, nil
	}
	if token != "" {
		return "", fmt.Errorf("only one of %s and %s may be set", conf.QueryServerToken, conf.QueryServerTokenFile)
	}

	content, err := os.ReadFile(filepath.Clean(tokenFile))
	if err != nil {
		return "", fmt.Errorf("failed to read query server token: %w", err)
	}
	token = strings.TrimSpace(string(content))
	if token == "" {
		return "", errors.New("query server token file " + tokenFile + " is empty")
	}
	return token, nil
}
