# goDB Query API

This package exposes methods to query the data stored in goDB.

## Example

To access the data captured by goProbe (stored at the default location) from your own application, you can use the following to get started:

```golang
func main() {
     // set query output(s) redirection (default is os.Stdout). You can use multiple io.Writers here
     ctx := context.Background()
     outputs := os.Stderr

     args := query.NewArgs("sip,dip", "eth0",
        query.WithSortAscending(),
        query.WithCondition("dport eq 443"),
     )

     // prepare the statement (e.g. parse args and setup query parameters).
     // This example assumes that you are querying against goDB
     stmt, err := args.Prepare(output)
     if err != nil {
          fmt.Fprintf(os.Stderr, "couldn't prepare statement: %s\n", err)
          os.Exit(1)
     }

     // execute statement
     err = engine.NewQueryRunner().Run(ctx, stmt)
     if err != nil {
          fmt.Fprintf(os.Stderr, "query failed: %s\n", err)
          os.Exit(1)
     }
}
```

For a more complete overview, please consult the documentation.
