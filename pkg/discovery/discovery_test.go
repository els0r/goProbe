package discovery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
)

// Response stores the ntm API responses
type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func TestMarshalConfig(t *testing.T) {
	var tests = []struct {
		name string
		cfg  *Config
	}{
		{
			"full config",
			&Config{
				Identifier: "test_ident",
				Versions:   []string{"v1", "v2"},
				Keys:       []string{"key1", "key2"},
				Endpoint:   "localhost:58000",
			},
		},
		{
			"only identifier and endpoint",
			&Config{
				Identifier: "test_ident",
				Endpoint:   "localhost:58000",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := &bytes.Buffer{}

			// check marshalling
			err := json.NewEncoder(b).Encode(test.cfg)
			if err != nil {
				t.Fatalf("could not marshal config: %s", err)
			}

			t.Logf("config in json: %s", b)
		})
	}
}

func TestUnmarshalConfig(t *testing.T) {
	var tests = []struct {
		name      string
		cfgJSON   string
		cfgExpect *Config
	}{
		{
			"full config",
			`{"identifier":"test_ident","versions":["v1","v2"],"endpoint":"localhost:58000","keys":["key1","key2"]}`,
			&Config{
				Identifier: "test_ident",
				Versions:   []string{"v1", "v2"},
				Keys:       []string{"key1", "key2"},
				Endpoint:   "localhost:58000",
			},
		},
		{
			"only endpoint",
			`{"endpoint":"localhost:58000"}`,
			&Config{
				Endpoint: "localhost:58000",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.cfgJSON)

			// unmarshal config
			cfg := new(Config)
			err := json.NewDecoder(r).Decode(cfg)
			if err != nil {
				t.Fatalf("could not unmarshal config: %s", err)
			}

			// check if structs are equal
			if !reflect.DeepEqual(cfg, test.cfgExpect) {
				t.Fatalf("config mismatch. Got: %s, Expect: %s", cfg, test.cfgExpect)
			}
		})
	}
}

func TestGet(t *testing.T) {
	var tests = []struct {
		name           string
		id             string
		config         *Config
		expectedStatus int
	}{
		{
			"config exists under path",
			"some_id",
			&Config{
				Identifier: "some_id",
				Endpoint:   "192.168.0.254:58000",
				Versions:   []string{"v1"},
			},
			200,
		},
		{
			"config does not exist",
			"some_id",
			nil,
			404,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json; charset=UTF-8")
					w.WriteHeader(test.expectedStatus)

					if test.config != nil {

						resp := &Response{Message: "test server: get", Data: test.config}

						err := json.NewEncoder(w).Encode(resp)
						if err != nil {
							return
						}
					}
				},
			))
			defer s.Close()

			c := NewClient(s.URL, WithUserAgent("test_client"))

			var (
				cfgGot *Config
				err    error
			)
			cfgGot, err = c.Get(test.id)
			if test.expectedStatus == 200 {
				if err != nil {
					t.Fatalf("failed to retrieve config: %s", err)
				}
				if !reflect.DeepEqual(test.config, cfgGot) {
					t.Fatalf("did not get expected config; got: %v", cfgGot)
				}
			} else {
				if err == nil {
					t.Fatalf("expected %d error", test.expectedStatus)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	var tests = []struct {
		name           string
		config         *Config
		expectedStatus int
	}{
		{
			"config created under path",
			&Config{
				Identifier: "some_id",
				Endpoint:   "192.168.0.254:58000",
				Versions:   []string{"v1"},
			},
			201,
		},
		{
			"config is nil",
			nil,
			500,
		},
		{
			"config exists already",
			&Config{
				Identifier: "some_id",
				Endpoint:   "192.168.0.254:58000",
				Versions:   []string{"v1"},
			},
			200,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json; charset=UTF-8")

					// read response body and check if the source data is okay
					cfg := new(Config)
					err := json.NewDecoder(r.Body).Decode(cfg)
					if err != nil {
						w.WriteHeader(500)
						return
					}

					if test.config != nil {
						resp := &Response{Message: "test server: create", Data: test.config}

						err := json.NewEncoder(w).Encode(resp)
						if err != nil {
							w.WriteHeader(500)
							return
						}
						w.WriteHeader(test.expectedStatus)
						return
					}
				},
			))
			defer s.Close()

			c := NewClient(s.URL, WithUserAgent("test_client"))

			var (
				cfgGot *Config
				err    error
			)
			cfgGot, err = c.Create(test.config)
			if test.expectedStatus == 200 || test.expectedStatus == 201 {
				if err != nil {
					t.Fatalf("failed to post config: %s", err)
				}
				if !reflect.DeepEqual(test.config, cfgGot) {
					t.Fatalf("did not get expected config; got: %v", cfgGot)
				}
			} else {
				if err == nil {
					t.Fatalf("expected %d error", test.expectedStatus)
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	var tests = []struct {
		name           string
		configs        []*Config
		expectedStatus int
	}{
		{
			"list all empty",
			nil,
			200,
		},
		{
			"list all two",
			[]*Config{
				&Config{
					Identifier: "id_1",
					Endpoint:   "192.168.0.253:58000",
					Versions:   []string{"v1"},
				},
				&Config{
					Identifier: "id_2",
					Endpoint:   "192.168.0.254:58000",
					Versions:   []string{"v1"},
				},
			},
			200,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json; charset=UTF-8")
					w.WriteHeader(test.expectedStatus)
					resp := &Response{Message: "test server: list", Data: test.configs}

					err := json.NewEncoder(w).Encode(resp)
					if err != nil {
						return
					}
				},
			))
			defer s.Close()

			c := NewClient(s.URL, WithUserAgent("test_client"))

			var (
				cfgsGot []*Config
				err     error
			)
			cfgsGot, err = c.List()
			if test.expectedStatus == 200 {
				if err != nil {
					t.Fatalf("failed to retrieve config: %s", err)
				}
				if !reflect.DeepEqual(test.configs, cfgsGot) {
					t.Fatalf("did not get expected config; got: %v", cfgsGot)
				}
			} else {
				if err == nil {
					t.Fatalf("expected %d error", test.expectedStatus)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	var tests = []struct {
		name           string
		expectedStatus int
	}{
		{
			"probe exists and was deleted",
			204,
		},
		{
			"probe does not exist",
			404,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json; charset=UTF-8")
					w.WriteHeader(test.expectedStatus)
				},
			))
			defer s.Close()

			c := NewClient(s.URL, WithUserAgent("test_client"))

			err := c.Delete("some_id")
			if test.expectedStatus == 204 {
				if err != nil {
					t.Fatalf("delete failed: %s", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected %d error", test.expectedStatus)
				}
			}
		})
	}
}

func TestMakeConfig(t *testing.T) {
	var tests = []struct {
		name        string
		input       *capconfig.Config
		expectedCfg *Config
	}{
		{
			"valid config",
			&capconfig.Config{
				API: capconfig.APIConfig{
					Keys: []string{"key1", "key2"},
					Discovery: &capconfig.DiscoveryConfig{
						Endpoint:   "localhost:6060",
						Identifier: "test_id",
					},
				},
			},
			&Config{
				Identifier: "test_id",
				Endpoint:   "localhost:6060",
				Keys:       []string{"key1", "key2"},
				Versions:   []string{"v1"},
			},
		},
		{
			"no service discovery",
			&capconfig.Config{
				API: capconfig.APIConfig{
					Keys: []string{"key1", "key2"},
				},
			},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfgGot := MakeConfig(test.input)
			if !reflect.DeepEqual(test.expectedCfg, cfgGot) {
				t.Fatalf("mismatched config; got: %s", cfgGot)
			}
		})
	}
}
