package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var tests = []struct {
	name       string
	shouldFail bool
}{
	{"wrong discovery config", true},
	{"valid configuration (api, logging, discovery)", false},
	{"valid configuration (api, logging)", false},
	{"fails on API section", true},
	{"missing iface config", true},
	{"missing server addr", true},
	{"insecure API key", true},
	{"faulty json", true},
	// this is ok, since the default DB path is assigned
	{"empty DB path", false},
	{"broken interface config", true},
	{"negative timeout", true},
	{"unknown encoder", true},
}

func TestValidate(t *testing.T) {
	// run tests
	for i, test := range tests {
		// run each case as a sub test
		t.Run(test.name, func(t *testing.T) {
			// create reader to parse config
			path := fmt.Sprintf("testdata/%d.json", i)
			r, err := os.OpenFile(filepath.Clean(path), os.O_RDONLY, 0600)

			assert.Nil(t, err, "failed to open test file at %s: %v", path, err)

			// parse config
			cfg, err := Parse(r)
			if test.shouldFail {
				if err == nil {
					t.Log(cfg)
					t.Fatalf("[%d] config parsing should have failed but didn't", i)
				}
				t.Logf("[%d] provoked expected error: %s", i, err)
				return
			}
			if err != nil {
				t.Fatalf("[%d] couldn't parse config: %s", i, err)
			}

			p := cfg.DB.Path
			if p == "" {
				t.Fatalf("[%d] the config DB path should never be empty after parsing a config", i)
			}
		})
	}
}
