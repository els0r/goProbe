package hosts

import "context"

// ID identifies a remote goProbe instance to query (host, FQDN, UUID, etc.).
// If you already have an ID type elsewhere, feel free to replace this.
type ID string

// Hosts is a slice of ID values, used for returning multiple Host IDs via a resolver
type Hosts []ID

// Resolver returns the set of IDs that the distributed querier should contact
type Resolver interface {
	Resolve(ctx context.Context, query string) (Hosts, error)
}
