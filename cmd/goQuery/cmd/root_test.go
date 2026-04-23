package cmd

import (
	"testing"

	"github.com/els0r/goProbe/v4/cmd/goQuery/pkg/conf"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRequireDBPathIfLocalOperation(t *testing.T) {
	tests := []struct {
		name            string
		dbPath          string
		queryServerAddr string
		isListOperation bool
		expectedErr     error
	}{
		{
			name:            "local query without db path fails",
			dbPath:          "",
			queryServerAddr: "",
			isListOperation: false,
			expectedErr:     errorEmptyDBPath,
		},
		{
			name:            "local query with db path succeeds",
			dbPath:          "/var/lib/goprobe/db",
			queryServerAddr: "",
			isListOperation: false,
			expectedErr:     nil,
		},
		{
			name:            "query server mode without db path succeeds",
			dbPath:          "",
			queryServerAddr: "127.0.0.1:8146",
			isListOperation: false,
			expectedErr:     nil,
		},
		{
			name:            "list operation without db path fails",
			dbPath:          "",
			queryServerAddr: "127.0.0.1:8146",
			isListOperation: true,
			expectedErr:     errorEmptyDBPath,
		},
		{
			name:            "whitespace db path treated as empty",
			dbPath:          "  ",
			queryServerAddr: "",
			isListOperation: false,
			expectedErr:     errorEmptyDBPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requireDBPathIfLocalOperation(tt.dbPath, tt.queryServerAddr, tt.isListOperation)
			assert.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestInitConfigReadsDBPathFromConfigFile(t *testing.T) {
	viper.Reset()
	viper.Set(conf.QueryDBPath, "/var/lib/goprobe/db")

	err := requireDBPathIfLocalOperation(viper.GetString(conf.QueryDBPath), "", false)
	assert.NoError(t, err)
}
