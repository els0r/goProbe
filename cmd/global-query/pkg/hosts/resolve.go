package hosts

import (
	"context"
	"sort"
	"strings"
)

// Hosts stores a list of host strings
type Hosts []string

// ResolverType enumerates the supported host resolvers
type ResolverType string

const (

	// StringResolverType denotes a simple string resolver type
	StringResolverType ResolverType = "string"
)

// Resolver returns a list of hosts based on the query string
type Resolver interface {
	Resolve(ctx context.Context, query string) (Hosts, error)
}

// StringResolver transforms a comma-separated list of hosts into an array. Sorting is
// enabled by default
type StringResolver struct {
	sorted bool
}

// NewStringResolver creates a new string-based hosts resolver
func NewStringResolver(sorted bool) *StringResolver {
	return &StringResolver{sorted: sorted}
}

// Resolve accepts a comma-separated list of hosts and returns them sorted and deduplicated in a Hosts array
func (s *StringResolver) Resolve(_ context.Context, query string) (hostList Hosts, err error) {
	var hostMap = make(map[string]struct{})
	for _, h := range strings.Split(strings.TrimSpace(query), ",") {
		_, exists := hostMap[h]
		if h != "" && !exists {
			hostMap[h] = struct{}{}
		}
	}
	hostList = make(Hosts, len(hostMap))
	i := 0
	for k := range hostMap {
		hostList[i] = k
		i++
	}
	sort.Slice(hostList, func(i, j int) bool {
		return hostList[i] < hostList[j]
	})
	return hostList, nil
}
