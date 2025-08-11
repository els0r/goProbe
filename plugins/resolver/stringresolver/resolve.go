package stringresolver

import (
	"context"
	"sort"
	"strings"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins"
)

// Resolver is a resolver for string-based host queries
type Resolver struct {
	sorted bool
}

// Resolve accepts a comma-separated list of hosts and returns them sorted and deduplicated in a Hosts array
func (r *Resolver) Resolve(_ context.Context, query string) (hostList hosts.Hosts, err error) {
	var hostMap = make(map[hosts.ID]struct{})
	for _, h := range strings.Split(strings.TrimSpace(query), ",") {
		id := strings.TrimSpace(h)
		_, exists := hostMap[hosts.ID(id)]
		if id != "" && !exists {
			hostMap[hosts.ID(id)] = struct{}{}
		}
	}
	hostList = make(hosts.Hosts, len(hostMap))
	i := 0
	for k := range hostMap {
		hostList[i] = k
		i++
	}
	if r.sorted {
		sort.SliceStable(hostList, func(i, j int) bool {
			return hostList[i] < hostList[j]
		})
	}
	return hostList, nil
}

func init() {
	plugins.RegisterResolver("string", func(_ context.Context, _ string) (hosts.Resolver, error) {
		return &Resolver{sorted: true}, nil
	})
}
