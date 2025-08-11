package staticresolver

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins"
)

// Config file schema: {"ids": ["hostA","hostB", ...]}
type Config struct {
	IDs []string `json:"ids"`
}

// Resolver holds the pre-loaded host IDs from the configuration
type Resolver struct {
	ids hosts.Hosts
}

// Resolve hosts based on the pre-loaded hosts in the configuration
func (r *Resolver) Resolve(_ context.Context, _ string) (hosts.Hosts, error) {
	return r.ids, nil
}

func load(cfgPath string) (*Resolver, error) {
	if cfgPath == "" {
		// Keep behavior graceful if no config is supplied: empty list.
		return &Resolver{ids: nil}, nil
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if len(c.IDs) == 0 {
		return nil, errors.New("static resolver: config has no ids")
	}
	out := make(hosts.Hosts, 0, len(c.IDs))
	for _, s := range c.IDs {
		out = append(out, hosts.ID(s))
	}
	return &Resolver{ids: out}, nil
}

func init() {
	plugins.RegisterResolver("static", func(_ context.Context, cfgPath string) (hosts.Resolver, error) {
		return load(cfgPath)
	})
}
