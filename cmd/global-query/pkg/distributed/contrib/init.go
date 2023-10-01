//go:build !contrib

package contrib

// enumerates the default plugin list
import (
	_ "github.com/els0r/goProbe/cmd/global-query/pkg/distributed/contrib/querier/apiclient"
)
