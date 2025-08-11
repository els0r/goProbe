package resolver

// enumerates the default resolver plugin list
import (
	_ "github.com/els0r/goProbe/v4/plugins/resolver/staticresolver"
	_ "github.com/els0r/goProbe/v4/plugins/resolver/stringresolver"
)
