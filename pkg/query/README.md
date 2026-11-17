# goDB Query API

This package exposes methods to query the data stored in goDB.

## Example

To access the data captured by goProbe (stored at the default location) from your own application, you can use the following to get started:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/els0r/goProbe/v4/pkg/defaults"
	"github.com/els0r/goProbe/v4/pkg/goDB/engine"
	"github.com/els0r/goProbe/v4/pkg/query"
)

func main() {
	ctx := context.Background()

	// build the query arguments (defaults are applied by NewArgs)
	args := query.NewArgs("sip,dip", "eth0",
		query.WithSortAscending(),
		query.WithCondition("dport eq 443"),
	)

	// optional: route the printed results somewhere other than os.Stdout.
	// You can pass multiple io.Writers here
	args.AddOutputs(os.Stderr)

	// validate the arguments and create the executable statement (needed for printing)
	stmt, err := args.Prepare()
	if err != nil {
		fmt.Fprintf(os.Stderr, "couldn't prepare statement: %s\n", err)
		os.Exit(1)
	}

	// execute the query against a local goDB (the runner prepares the args itself)
	res, err := engine.NewQueryRunner(defaults.DBPath).Run(ctx, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query failed: %s\n", err)
		os.Exit(1)
	}

	// post-process (time binning, row limit) and print the results
	if err := stmt.PostProcess(ctx, res); err != nil {
		fmt.Fprintf(os.Stderr, "couldn't post-process results: %s\n", err)
		os.Exit(1)
	}
	if err := stmt.Print(ctx, res); err != nil {
		fmt.Fprintf(os.Stderr, "couldn't print results: %s\n", err)
		os.Exit(1)
	}
}
```

For the full API see https://pkg.go.dev/github.com/els0r/goProbe/v4/pkg/query.
