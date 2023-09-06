package query

import "time"

// Option allows to modify an existing Args container
type Option func(*Args)

// WithCondition sets the condition argument
func WithCondition(c string) Option { return func(a *Args) { a.Condition = c } }

// WithDirectionIn considers the incoming flows
func WithDirectionIn() Option { return func(a *Args) { a.In = true } }

// WithDirectionOut considers the outgoing flows
func WithDirectionOut() Option { return func(a *Args) { a.Out = true } }

// WithDirectionSum adds both directions
func WithDirectionSum() Option { return func(a *Args) { a.Sum = true } }

// WithFirst sets the first timestamp to consider
func WithFirst(f string) Option { return func(a *Args) { a.First = f } }

// WithLast sets the last timestampt to consider
func WithLast(l string) Option { return func(a *Args) { a.Last = l } }

// WithFormat sets the output format
func WithFormat(f string) Option { return func(a *Args) { a.Format = f } }

// WithSortBy sets by which parameter should be sorted
func WithSortBy(s string) Option { return func(a *Args) { a.SortBy = s } }

// WithNumResults sets how many rows are returned
func WithNumResults(n int) Option { return func(a *Args) { a.NumResults = n } }

// WithSortAscending sorts rows ascending
func WithSortAscending() Option { return func(a *Args) { a.SortAscending = true } }

// WithList sets the list parameter (only lists interfaces)
func WithList() Option { return func(a *Args) { a.List = true } }

// WithVersion sets the version parameter (print version and exit)
func WithVersion() Option { return func(a *Args) { a.Version = true } }

// WithResolve enables reverse lookups of IPs
func WithResolve() Option { return func(a *Args) { a.DNSResolution.Enabled = true } }

// WithResolveTimeout sets the timeout for reverse lookups (in seconds)
func WithResolveTimeout(t time.Duration) Option { return func(a *Args) { a.DNSResolution.Timeout = t } }

// WithResolveRows sets the amount of rows for which lookups should be attempted
func WithResolveRows(r int) Option { return func(a *Args) { a.DNSResolution.MaxRows = r } }

// WithMaxMemPct is an advanced parameter to restrict system memory usage to a fixed percentage of the available memory during query processing
func WithMaxMemPct(m int) Option { return func(a *Args) { a.MaxMemPct = m } }

// WithCaller sets the name of the program/tool calling the query
func WithCaller(c string) Option { return func(a *Args) { a.Caller = c } }
