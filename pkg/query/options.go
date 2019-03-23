package query

// Option allows to modify an existing Args container
type Option func(*Args)

func WithCondition(c string) Option   { return func(a *Args) { a.Condition = c } }
func WithDirectionIn() Option         { return func(a *Args) { a.In = true } }
func WithDirectionOut() Option        { return func(a *Args) { a.Out = true } }
func WithDirectionSum() Option        { return func(a *Args) { a.Sum = true } }
func WithFirst(f string) Option       { return func(a *Args) { a.First = f } }
func WithLast(l string) Option        { return func(a *Args) { a.Last = l } }
func WithFormat(f string) Option      { return func(a *Args) { a.Format = f } }
func WithSortBy(s string) Option      { return func(a *Args) { a.SortBy = s } }
func WithNumResults(n int) Option     { return func(a *Args) { a.NumResults = n } }
func WithExternal() Option            { return func(a *Args) { a.External = true } }
func WithSortAscending() Option       { return func(a *Args) { a.SortAscending = true } }
func WithOutput(o string) Option      { return func(a *Args) { a.Output = o } }
func WithList() Option                { return func(a *Args) { a.List = true } }
func WithVersion() Option             { return func(a *Args) { a.Version = true } }
func WithResolve() Option             { return func(a *Args) { a.Resolve = true } }
func WithResolveTimeout(t int) Option { return func(a *Args) { a.ResolveTimeout = t } }
func WithResolveRows(r int) Option    { return func(a *Args) { a.ResolveRows = r } }
func WithDBPath(p string) Option      { return func(a *Args) { a.DBPath = p } }
func WithMaxMemPct(m int) Option      { return func(a *Args) { a.MaxMemPct = m } }
func WithCaller(c string) Option      { return func(a *Args) { a.Caller = c } }
