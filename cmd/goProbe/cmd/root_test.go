package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gpconf "github.com/els0r/goProbe/v4/cmd/goProbe/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestNewRootCmd(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		configFile      string
		configContent   string
		expectedCfg     *gpconf.Config
		expectedCfgFile string
		expectError     bool
	}{
		{
			name: "all flags set",
			args: []string{
				"--db.path=/tmp/test/db",
				"--db.encoder_type=lz4",
				"--db.permissions=0755",
				"--autodetection.enabled=true",
				"--autodetection.exclude=lo,docker0",
				"--syslog_flows=true",
				"--api.addr=127.0.0.1:8080",
				"--api.metrics=true",
				"--api.profiling=true",
				"--api.keys=59436af63ebf98a39de763d56220edb90267debaa4180b864811c7a44ad35bc8",
				"--api.query_rate_limit.max_req_per_sec=10.5",
				"--api.query_rate_limit.max_burst=1",
				"--api.query_rate_limit.max_concurrent=1",
				"--local_buffers.size_limit=1048576",
				"--local_buffers.num_buffers=4",
			},
			expectedCfg: &gpconf.Config{
				DB: gpconf.DBConfig{
					Path:        "/tmp/test/db",
					EncoderType: "lz4",
					Permissions: 0755,
				},
				AutoDetection: gpconf.AutoDetectionConfig{
					Enabled: true,
					Exclude: []string{"lo", "docker0"},
				},
				SyslogFlows: true,
				API: gpconf.APIConfig{
					Addr:                "127.0.0.1:8080",
					Metrics:             true,
					DisableIfaceMetrics: false,
					Profiling:           true,
					Timeout:             0,
					Keys:                []string{"59436af63ebf98a39de763d56220edb90267debaa4180b864811c7a44ad35bc8"},
					QueryRateLimit: gpconf.QueryRateLimitConfig{
						MaxReqPerSecond: rate.Limit(10.5),
						MaxBurst:        1,
						MaxConcurrent:   1,
					},
				},
				LocalBuffers: gpconf.LocalBufferConfig{
					SizeLimit:  1048576,
					NumBuffers: 4,
				},
				Interfaces: make(gpconf.Ifaces),
			},
			expectedCfgFile: "",
			expectError:     false,
		},
		{
			name: "basic invocation (db path + autodetection)",
			args: []string{
				"--db.path=/var/lib/goprobe/db",
				"--autodetection.enabled=true",
			},
			expectedCfg: &gpconf.Config{
				DB: gpconf.DBConfig{
					Path: "/var/lib/goprobe/db",
				},
				AutoDetection: gpconf.AutoDetectionConfig{
					Enabled: true,
					Exclude: []string{},
				},
				API: gpconf.APIConfig{
					Keys: []string{},
				},
			},
			expectedCfgFile: "",
			expectError:     false,
		},
		{
			name:       "config file flag",
			args:       []string{},
			configFile: "test-config.yaml",
			configContent: `---
db:
  path: /test/db/path
  encoder_type: lz4
  permissions: 0750
syslog_flows: true
api:
  addr: 192.168.1.1:8888
  metrics: true
interfaces:
  eth0:
    promisc: true
    ring_buffer:
      num_blocks: 8
      block_size: 2048
`,
			expectedCfg: &gpconf.Config{
				DB: gpconf.DBConfig{
					Path:        "/test/db/path",
					EncoderType: "lz4",
					Permissions: 0750,
				},
				AutoDetection: gpconf.AutoDetectionConfig{
					Enabled: false,
					Exclude: []string{},
				},
				SyslogFlows: true,
				API: gpconf.APIConfig{
					Addr:                "192.168.1.1:8888",
					Metrics:             true,
					DisableIfaceMetrics: false,
					Profiling:           false,
					Timeout:             0,
					Keys:                []string{},
					QueryRateLimit: gpconf.QueryRateLimitConfig{
						MaxReqPerSecond: 0,
						MaxBurst:        0,
						MaxConcurrent:   0,
					},
				},
				Interfaces: gpconf.Ifaces{
					"eth0": gpconf.CaptureConfig{
						Promisc: true,
						RingBuffer: &gpconf.RingBufferConfig{
							NumBlocks: 8,
							BlockSize: 2048,
						},
					},
				},
			},
			expectedCfgFile: "",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper for each test
			viper.Reset()

			// Setup test config file if needed
			var tempDir string
			var configPath string
			if tt.configFile != "" {
				var err error
				tempDir, err = os.MkdirTemp("", "goprobe-test-*")
				require.NoError(t, err)
				t.Cleanup(func() {
					require.Nil(t, os.RemoveAll(tempDir))
				})

				configPath = filepath.Join(tempDir, tt.configFile)
				err = os.WriteFile(configPath, []byte(tt.configContent), 0644)
				require.NoError(t, err)

				// Add config flag to args
				tt.args = append([]string{"--config=" + configPath}, tt.args...)
			}

			// Track if runFunc was called and capture config values
			var capturedCfg *gpconf.Config
			runFuncCalled := false

			testRunFunc := func(ctx context.Context, cfg *gpconf.Config) error {
				runFuncCalled = true
				capturedCfg = cfg
				return nil
			}

			// Create root command
			rootCmd, err := newRootCmd(testRunFunc)
			require.NoError(t, err)
			require.NotNil(t, rootCmd)

			// Set args and execute
			rootCmd.SetArgs(tt.args)
			err = rootCmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, runFuncCalled, "runFunc should have been called")

			// Verify config values
			require.NotNil(t, capturedCfg)

			// DB config
			assert.Equal(t, tt.expectedCfg.DB.Path, capturedCfg.DB.Path, "DB.Path mismatch")
			assert.Equal(t, tt.expectedCfg.DB.EncoderType, capturedCfg.DB.EncoderType, "DB.EncoderType mismatch")
			assert.Equal(t, tt.expectedCfg.DB.Permissions, capturedCfg.DB.Permissions, "DB.Permissions mismatch")

			// AutoDetection config
			assert.Equal(t, tt.expectedCfg.AutoDetection.Enabled, capturedCfg.AutoDetection.Enabled, "AutoDetection.Enabled mismatch")
			assert.Equal(t, tt.expectedCfg.AutoDetection.Exclude, capturedCfg.AutoDetection.Exclude, "AutoDetection.Exclude mismatch")

			// SyslogFlows
			assert.Equal(t, tt.expectedCfg.SyslogFlows, capturedCfg.SyslogFlows, "SyslogFlows mismatch")

			// API config
			require.NotNil(t, capturedCfg.API, "API should not be nil")
			assert.EqualValues(t, tt.expectedCfg.API, capturedCfg.API, "API config mismatch")

			// LocalBuffers config
			require.NotNil(t, capturedCfg.LocalBuffers, "LocalBuffers should not be nil")
			assert.Equal(t, tt.expectedCfg.LocalBuffers.SizeLimit, capturedCfg.LocalBuffers.SizeLimit, "LocalBuffers.SizeLimit mismatch")
			assert.Equal(t, tt.expectedCfg.LocalBuffers.NumBuffers, capturedCfg.LocalBuffers.NumBuffers, "LocalBuffers.NumBuffers mismatch")

			// Interfaces
			assert.NotNil(t, capturedCfg.Interfaces, "Interfaces should not be nil")
			if tt.expectedCfg.Interfaces != nil {
				assert.Equal(t, len(tt.expectedCfg.Interfaces), len(capturedCfg.Interfaces), "Interfaces length mismatch")
				for ifaceName, expectedIface := range tt.expectedCfg.Interfaces {
					actualIface, exists := capturedCfg.Interfaces[ifaceName]
					assert.True(t, exists, "Interface %s should exist", ifaceName)
					assert.Equal(t, expectedIface, actualIface, "Interface %s config mismatch", ifaceName)
				}
			}
		})
	}
}
