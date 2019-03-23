package config

import (
	"strings"
	"testing"
)

var tests = []struct {
	shouldFail bool
	cfg        string
}{
	{
		false,
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
},
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "6060",
    "request_logging" : false
}
}`,
	},
	{
		true,
		// should fail on API section
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    },
    "en1" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
},
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "6060",
    "request_logging" : true,
    "keys" : [
        "da53ae3fb482db63d9606a9324a694bf51f7ad47623c04ab7b97a811f2a78e05",
        "9e3b84ae1437a73154ac5c48a37d5085a3f6e68621b56b626f81620de271a2f6"
    ]
}
}`,
	},
	{
		true,
		// fails due to missing iface config
		`{
"db_path" : "/opt/ntm/goProbe/db",
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "6060",
    "request_logging" : false
}
}`,
	},
	{
		true,
		// fails due to empty API port
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
},
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "",
    "request_logging" : false
}
}`,
	},
	{
		true,
		// fails due to insecure API key
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
},
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "6060",
    "request_logging" : false,
    "keys" : [ "i am too short" ]
}
}`,
	},
	{
		true,
		// fails due to faulty json
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    },
    "en1" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
},
`,
	},
	{
		true,
		// fails due to empty DB path
		`{
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    },
    "en1" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
}
}
`,
	},
	{
		true,
		// fails due to broken interface configuration
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 0,
        "promisc" : true
    }
},
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "6060",
    "request_logging" : false
}
}`,
	},
	{
		true,
		// fails due to negative timeout
		`{
"db_path" : "/opt/ntm/goProbe/db",
"interfaces" : {
    "en0" : {
        "bpf_filter" : "not arp and not icmp",
        "buf_size" : 2097152,
        "promisc" : true
    }
},
"logging" : {
    "destination" : "console",
    "level" : "debug"
},
"api" : {
    "port" : "6060",
    "request_logging" : false,
    "request_timeout" : -1
}
}`,
	},
}

func TestValidate(t *testing.T) {

	// run tests
	for i, test := range tests {
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
			continue
		} else {
			if err != nil {
				t.Fatalf("[%d] couldn't parse config: %s", i, err)
			}
		}

		p := RuntimeDBPath()
		if p == "" {
			t.Fatalf("[%d] the runtime DB path should never be empty after parsing a config", i)
		}
	}
}
