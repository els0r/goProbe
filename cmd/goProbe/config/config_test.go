package config

import (
	"sort"
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
				API: APIConfig{
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
		{"missing API addr (don't parse API if not set)",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				API: APIConfig{
					Keys: []string{"testtesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttesttest"},
				},
			},
			nil,
		},
		{"invalid / missing rate limit continuous rate value",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"eth0": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
				API: APIConfig{
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
				API: APIConfig{
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
				API: APIConfig{
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
		{"regex matcher interface valid",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					"/eth[0-9]/": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
			},
			nil,
		},
		{"regex matcher requires disable when autodetect present",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					InterfaceAuto: CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
					"/eth[0-9]/": CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
				},
			},
			errorIfaceMustBeDisabledWithAuto,
		},
		{"regex matcher disabled with autodetect",
			&Config{
				DB: DBConfig{Path: defaults.DBPath},
				Interfaces: Ifaces{
					InterfaceAuto: CaptureConfig{
						RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
					},
					"/eth[0-9]/": CaptureConfig{
						Disable: true,
					},
				},
			},
			nil,
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

func TestIsRegexpInterfaceMatcher(t *testing.T) {
	var tests = []struct {
		name     string
		iface    IfaceName
		expected bool
	}{
		{
			name:     "plain interface name",
			iface:    "eth0",
			expected: false,
		},
		{
			name:     "no closing slash",
			iface:    "/eth[0-9]",
			expected: false,
		},
		{
			name:     "no opening slash",
			iface:    "eth[0-9]/",
			expected: false,
		},
		{
			name:     "valid regex",
			iface:    "/eth[0-9]/",
			expected: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, IsRegexpInterfaceMatcher(test.iface))
		})
	}
}

func TestHasRegexpMatching(t *testing.T) {
	var tests = []struct {
		name               string
		ifaces             Ifaces
		expectedMatch      string
		expectedFound      bool
		expectedErrContain string
	}{
		{
			name: "no regex matchers",
			ifaces: Ifaces{
				"eth0": CaptureConfig{
					RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
				},
			},
			expectedMatch:      "",
			expectedFound:      false,
			expectedErrContain: "",
		},
		{
			name: "single regex matcher",
			ifaces: Ifaces{
				"/eth[0-9]/": CaptureConfig{
					RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
				},
			},
			expectedMatch:      "eth[0-9]",
			expectedFound:      true,
			expectedErrContain: "",
		},
		{
			name: "multiple regex matchers",
			ifaces: Ifaces{
				"/eth[0-9]/": CaptureConfig{
					RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
				},
				"/wlan[0-9]/": CaptureConfig{
					RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
				},
			},
			expectedMatch:      "eth[0-9]|wlan[0-9]",
			expectedFound:      true,
			expectedErrContain: "",
		},
		{
			name: "invalid regex matcher",
			ifaces: Ifaces{
				"/eth[0-9/": CaptureConfig{
					RingBuffer: &RingBufferConfig{BlockSize: 1024 * 1024, NumBlocks: 2},
				},
			},
			expectedMatch:      "eth[0-9",
			expectedFound:      true,
			expectedErrContain: "missing closing ]",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			matcher, found, err := test.ifaces.Matcher()
			if test.expectedErrContain != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrContain)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedFound, found)
			if test.expectedFound {
				// There may be multiple regex matchers, join them for comparison
				var patterns []string
				for re := range matcher.regexpMatchers {
					patterns = append(patterns, re.String())
				}
				// Sort patterns for deterministic comparison
				sort.Strings(patterns)
				assert.Equal(t, test.expectedMatch, strings.Join(patterns, "|"))
			} else {
				assert.NotNil(t, matcher)
				assert.Equal(t, 0, len(matcher.regexpMatchers))
			}
		})
	}
}
