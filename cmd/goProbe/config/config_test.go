package config

import (
	"strings"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/defaults"
	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	var tests = []struct {
		name        string
		input       *Config
		expectedErr error
	}{
		{"new", newDefault(), errorNoInterfacesSpecified},
		{"valid config",
			&Config{
				DB: DBConfig{
					Path: defaults.DBPath,
				},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{
							BlockSize: 1024 * 1024,
							NumBlocks: 2,
						},
					},
				},
			},
			nil,
		},
		{"valid config extended",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				Logging: LogConfig{Level: "debug", Encoding: "logfmt"},
				API: &APIConfig{
					Addr: "unix:/var/run/goprobe.sock",
					Keys: []string{"testtesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttest"},
				},
			},
			nil,
		},
		{"empty DB path",
			&Config{
				DB: DBConfig{},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{
							BlockSize: 1024 * 1024,
							NumBlocks: 2,
						},
					},
				},
			},
			errorEmptyDBPath,
		},
		{"no iface config provided",
			&Config{
				DB:         DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{},
			},
			errorNoInterfacesSpecified,
		},
		{"no ring buffer config",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						Promisc: true,
					},
				},
			},
			errorNoRingBufferConfig,
		},
		{"faulty ring buffer config",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{},
					},
				},
			},
			errorRingBufferBlockSize,
		},
		{"faulty ring buffer config: empty num blocks",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024},
					},
				},
			},
			errorRingBufferNumBlocks,
		},
		{"missing API addr",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				Logging: LogConfig{Level: "debug", Encoding: "logfmt"},
				API: &APIConfig{
					Keys: []string{"testtesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttest"},
				},
			},
			errorNoAPIAddrSpecified,
		},
		{"invalid / missing rate limit continuous rate value",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				Logging: LogConfig{Level: "debug", Encoding: "logfmt"},
				API: &APIConfig{
					Addr: "unix:/var/run/goprobe.sock",
					QueryRateLimit: QueryRateLimitConfig{
						MaxBurst: 3,
					},
				},
			},
			errorInvalidAPIQueryRateLimit,
		},
		{"invalid / missing rate limit burst rate value",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				Logging: LogConfig{Level: "debug", Encoding: "logfmt"},
				API: &APIConfig{
					Addr: "unix:/var/run/goprobe.sock",
					QueryRateLimit: QueryRateLimitConfig{
						MaxReqPerSecond: 1.0,
					},
				},
			},
			errorInvalidAPIQueryRateLimit,
		},
		{"max concurrent value",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				Logging: LogConfig{Level: "debug", Encoding: "logfmt"},
				API: &APIConfig{
					Addr: "unix:/var/run/goprobe.sock",
					QueryRateLimit: QueryRateLimitConfig{
						MaxConcurrent: -10,
					},
				},
			},
			errorInvalidAPIQueryRateLimit,
		},
		{"capture disabled with other settings",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						Disable:    true,
						Promisc:    true,
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
			},
			errorSettingsWithCaptureDisabled,
		},
		{"capture disabled without other settings",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						Disable: true,
					},
				},
			},
			nil,
		},
		{"autodetect interface validates configuration",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					InterfaceAuto: CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
					"eth0": CaptureConfig{Disable: true},
				},
			},
			nil,
		},
		{"autodetect requires other interfaces to be disabled",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					InterfaceAuto: CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
			},
			errorIfaceMustBeDisabledWithAuto,
		},
		{"autodetect missing ring buffer",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					InterfaceAuto: CaptureConfig{},
				},
			},
			errorNoRingBufferConfig,
		},
	}

	// run tests
	for _, test := range tests {
		test := test
		// run each case as a sub test
		t.Run(test.name, func(t *testing.T) {
			err := test.input.Validate()
			t.Log(test.input)
			assert.ErrorIs(t, err, test.expectedErr)
		})
	}
}

func TestParse(t *testing.T) {
	var tests = []struct {
		name        string
		input       string
		expectedErr error
	}{
		{"valid config YAML",
			`db:
  path: /var/lib/goprobe/goprobe.db
interfaces:
  eth0:
   promisc: true
   ring_buffer:
      block_size: 1048576
      num_blocks: 2
`,
			nil,
		},
		{
			"valid config JSON",
			`
{
	"db": {
		"path": "/var/lib/goprobe/goprobe.db"
	},
	"interfaces": {
		"eth0": {
			"promisc": true,
			"ring_buffer": {
				"block_size": 1048576,
				"num_blocks": 2
			}
		}
	}
}
`,
			nil,
		},
		{"malformed",
			`db`,
			errorUnmarshalConfig,
		},
		{"invalid",
			`db:`,
			// when parse is used, the default DB path is set. Hence, the next error
			// that can occur is the empty interface error
			errorNoInterfacesSpecified,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(test.input))
			assert.ErrorIs(t, err, test.expectedErr)
		})
	}
}
