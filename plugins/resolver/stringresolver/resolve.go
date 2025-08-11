package stringresolver

import (
	"context"
	"sort"
	"strings"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins"
)

// Type is the identifier of the string resolver
const Type = "string"

// Resolver is a resolver for string-based host queries
type Resolver struct {
	sorted bool
}

// NewResolver creates a new string resolver
func NewResolver(sorted bool) *Resolver {
	return &Resolver{sorted: sorted}
}

// Resolve accepts a comma-separated list of hosts and returns them sorted and deduplicated in a Hosts array
func (r *Resolver) Resolve(_ context.Context, query string) (hostList hosts.Hosts, err error) {
	var hostMap = make(map[hosts.ID]struct{})
	for _, h := range strings.Split(strings.TrimSpace(query), ",") {
		id := strings.TrimSpace(h)
		_, exists := hostMap[id]
		if id != "" && !exists {
			hostMap[id] = struct{}{}
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
	plugins.RegisterResolver(Type, func(_ context.Context, _ string) (hosts.Resolver, error) {
		return NewResolver(true), nil
	})
}
