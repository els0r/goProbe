package config

import (
	"strings"
	"testing"
)

var tests = []struct {
	name       string
	shouldFail bool
	cfg        string
}{
	{
		"wrong discovery config",
		true,
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false, "service_discovery" : { "registry": "192.168.1.1:5000" } } }`,
	},
	{
		"valid configuration (api, logging, discovery)",
		false,
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false, "service_discovery" : { "endpoint" : "localhost:6060", "registry": "192.168.1.1:5000", "probe_identifier": "test_probe" } } }`,
	},
	{
		"valid configuration (api, logging)",
		false,
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false } }`,
	},
	{
		"fails on API section",
		true,
		// should fail on API section
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true }, "en1" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : true, "keys" : [ "da53ae3fb482db63d9606a9324a694bf51f7ad47623c04ab7b97a811f2a78e05", "9e3b84ae1437a73154ac5c48a37d5085a3f6e68621b56b626f81620de271a2f6" ] } }`,
	},
	{
		"missing iface config",
		true,
		// fails due to missing iface config
		`{ "db_path" : "/usr/local/goProbe/db", "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false } }`,
	},
	{
		"missing API port",
		true,
		// fails due to empty API port
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "", "request_logging" : false } }`,
	},
	{
		"insecure API key",
		true,
		// fails due to insecure API key
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false, "keys" : [ "i am too short" ] } }`,
	},
	{
		"faulty json",
		true,
		// fails due to faulty json
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true }, "en1" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, `},
	{
		"empty DB path",
		true,
		// fails due to empty DB path
		`{ "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true }, "en1" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } } } `,
	},
	{
		"broken interface config",
		true,
		// fails due to broken interface configuration
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 0, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false } }`,
	},
	{
		"negative timeout",
		true,
		// fails due to negative timeout
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false, "request_timeout" : -1 } }`,
	},
	{
		"valid configuration (api, logging, discovery, encoder)",
		false,
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false, "service_discovery" : { "endpoint" : "localhost:6060", "registry": "192.168.1.1:5000", "probe_identifier": "test_probe" } }, "encoder_type": "lz4" }`,
	},
	{
		"unknown encoder",
		true,
		`{ "db_path" : "/usr/local/goProbe/db", "interfaces" : { "en0" : { "bpf_filter" : "not arp and not icmp", "buf_size" : 2097152, "promisc" : true } }, "logging" : { "destination" : "console", "level" : "debug" }, "api" : { "port" : "6060", "request_logging" : false, "service_discovery" : { "endpoint" : "localhost:6060", "registry": "192.168.1.1:5000", "probe_identifier": "test_probe" } }, "encoder_type": "iwillneverbesupported" }`,
	},
}

func TestValidate(t *testing.T) {

	// run tests
	for i, test := range tests {
		// run each case as a sub test
		t.Run(test.name, func(t *testing.T) {
			// create reader to parse config
			r := strings.NewReader(test.cfg)

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

			p := RuntimeDBPath()
			if p == "" {
				t.Fatalf("[%d] the runtime DB path should never be empty after parsing a config", i)
			}
		})
	}
}
