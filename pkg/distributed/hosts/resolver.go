// Package hosts defines the host resolution contracts for distributed queries
package hosts

import (
	"context"
	"sync"
)

// ID identifies a remote goProbe instance to query (host, FQDN, UUID, etc.).
// If you already have an ID type elsewhere, feel free to replace this.
type ID = string

// Hosts is a slice of ID values, used for returning multiple Host IDs via a resolver
type Hosts []ID

// Resolver returns the set of IDs that the distributed querier should contact
type Resolver interface {
	Resolve(ctx context.Context, query string) (Hosts, error)
}

// ResolverMap is a concurrency-safe map of named resolvers
type ResolverMap struct {
	mu        sync.RWMutex
	resolvers map[string]Resolver
}

// NewResolverMap creates a new ResolverMap
func NewResolverMap() *ResolverMap {
	return &ResolverMap{
		resolvers: make(map[string]Resolver),
	}
}

// Get returns the resolver for a given name in case it exists
func (rm *ResolverMap) Get(name string) (Resolver, bool) {
	rm.mu.RLock()
	resolver, ok := rm.resolvers[name]
	rm.mu.RUnlock()
	return resolver, ok
}

// Set sets the resolver for a given name
func (rm *ResolverMap) Set(name string, resolver Resolver) {
	rm.mu.Lock()
	rm.resolvers[name] = resolver
	rm.mu.Unlock()
}

// Delete deletes an entry from the map
func (rm *ResolverMap) Delete(name string) {
	rm.mu.Lock()
	delete(rm.resolvers, name)
	rm.mu.Unlock()
}
