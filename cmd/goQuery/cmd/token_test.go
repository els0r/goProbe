package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/els0r/goProbe/v4/cmd/goQuery/pkg/conf"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryServerToken(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		return path
	}

	tests := []struct {
		name      string
		token     string // set via config file / viper
		env       string // QUERY_SERVER_TOKEN
		tokenFile func(t *testing.T) string
		expected  string
		expectErr bool
	}{
		{name: "no credential", expected: ""},
		{name: "config", token: "from-config", expected: "from-config"},
		{name: "environment", env: "from-env", expected: "from-env"},
		{
			name:      "file with trailing newline",
			tokenFile: func(t *testing.T) string { return writeFile(t, "from-file\n") },
			expected:  "from-file",
		},
		{
			name:      "empty file",
			tokenFile: func(t *testing.T) string { return writeFile(t, " \n") },
			expectErr: true,
		},
		{
			name:      "missing file",
			tokenFile: func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent") },
			expectErr: true,
		},
		{
			name:      "token and file are ambiguous",
			token:     "from-config",
			tokenFile: func(t *testing.T) string { return writeFile(t, "from-file") },
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("QUERY_SERVER_TOKEN", tt.env)
			// an override, even an empty one, would shadow the environment
			if tt.token != "" {
				viper.Set(conf.QueryServerToken, tt.token)
			}
			t.Cleanup(func() { viper.Set(conf.QueryServerToken, nil) })
			tokenFile := ""
			if tt.tokenFile != nil {
				tokenFile = tt.tokenFile(t)
			}
			viper.Set(conf.QueryServerTokenFile, tokenFile)
			t.Cleanup(func() { viper.Set(conf.QueryServerTokenFile, "") })

			token, err := queryServerToken()
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, token)
		})
	}
}
