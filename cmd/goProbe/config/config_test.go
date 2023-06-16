package config

import (
	"testing"

	"github.com/els0r/goProbe/pkg/defaults"
	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	var tests = []struct {
		name        string
		input       *Config
		expectedErr error
	}{
		{"new", New(), errorNoInterfacesSpecified},
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
		// {"wrong discovery config", true},
		// {"valid configuration (api, logging, discovery)", false},
		// {"valid configuration (api, logging)", false},
		// {"fails on API section", true},
		// {"missing iface config", true},
		// {"missing server addr", true},
		// {"insecure API key", true},
		// {"faulty json", true},
		// // this is ok, since the default DB path is assigned
		// {"empty DB path", false},
		// {"broken interface config", true},
		// {"negative timeout", true},
		// {"unknown encoder", true},
	}

	// run tests
	for _, test := range tests {
		test := test
		// run each case as a sub test
		t.Run(test.name, func(t *testing.T) {
			err := test.input.Validate()
			assert.ErrorIs(t, err, test.expectedErr)
		})
	}
}
