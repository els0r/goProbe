# AGENTS.md

## Purpose
This guide is for agentic coding assistants working in this repository.
It defines practical build/lint/test commands and expected code style.

## Project Layout
- Language: Go (`go 1.25`)
- Module: `github.com/els0r/goProbe/v4`
- Workspace: `go.work` includes `.` and `./plugins/contrib`
- Primary binaries:
  - `./cmd/goProbe`
  - `./cmd/goQuery`
  - `./cmd/global-query`
  - `./cmd/gpctl`
  - `./cmd/goConvert`
- Primary code folders:
  - `pkg/` (API, capture, query, goDB, results, shared types)
  - `plugins/` (resolver/querier plugin registration)

## Cursor and Copilot Rules
Checked and not present in this repo:
- `.cursor/rules/`
- `.cursorrules`
- `.github/copilot-instructions.md`
No extra Cursor/Copilot instruction files are available to merge.

## Build Commands
Run from repository root.

### CI-equivalent builds
- `GOOS=linux GOARCH=amd64 go build -tags jsoniter -v ./...`
- `GOOS=linux GOARCH=amd64 go build -tags jsoniter,slimcap_nomock -v ./...`
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags jsoniter -v ./...`

### Build individual binaries
- `go build -tags jsoniter -o goProbe ./cmd/goProbe`
- `go build -tags jsoniter -o goQuery ./cmd/goQuery`
- `go build -tags jsoniter -o global-query ./cmd/global-query`
- `go build -tags jsoniter -o gpctl ./cmd/gpctl`
- `go build -tags jsoniter -o goConvert ./cmd/goConvert`

### Release-like builds (workflow parity)
- `GOOS=linux GOARCH=amd64 go build -a -tags jsoniter,slimcap_nomock -pgo=auto -o goProbe ./cmd/goProbe`
- `GOOS=linux GOARCH=amd64 go build -a -tags jsoniter -pgo=auto -o global-query ./cmd/global-query`
- `GOOS=linux GOARCH=amd64 go build -a -pgo=auto -o goQuery ./cmd/goQuery`
- `GOOS=linux GOARCH=amd64 go build -a -pgo=auto -o gpctl ./cmd/gpctl`
- `GOOS=linux GOARCH=amd64 go build -a -o goConvert ./cmd/goConvert`

## Test Commands

### Full and CI variants
- `go test -tags jsoniter -v ./...`
- `go test -tags jsoniter,slimcap_nomock -v ./...`
- `CGO_ENABLED=0 go test -tags jsoniter -v ./...`
- `go test -tags jsoniter -race -v ./...`

### Package-level testing
- `go test -tags jsoniter -v ./pkg/query`
- `go test -tags jsoniter -v ./pkg/goDB/engine`

### Run a single test (important)
- `go test -tags jsoniter -run '^TestPrepareArgs$' -v ./pkg/query`

### Run a single subtest
- `go test -tags jsoniter -run '^TestPrepareArgs/valid_query_args$' -v ./pkg/query`

### Discover test names in a package
- `go test -list . ./pkg/query`

### E2E package specifics (`pkg/e2etest`)
- Package defines custom test-binary flag `-skip-benchmarks`
- Pass custom flags after `-args`
- `go test -tags jsoniter -v ./pkg/e2etest -args -skip-benchmarks`
- `go test -tags jsoniter -run '^TestE2EBasic$' -v ./pkg/e2etest`

## Lint, Format, and Static Analysis
No dedicated lint workflow currently runs in CI, but dev setup indicates `golangci-lint` + `staticcheck`.
- `gofmt -w <changed-files>`
- `goimports -w <changed-files>`
- `go vet ./...`
- `golangci-lint run ./...`
If `golangci-lint` is unavailable, at minimum run `go test` and `go vet` for touched packages.

## Build Tags and Platform Notes
- Common tags: `jsoniter`, `slimcap_nomock`
- Compression toggles: `goprobe_noliblz4`, `goprobe_nolibzstd`
- CGO-free mode is supported with `CGO_ENABLED=0`
- OS-specific files are selected via `//go:build` tags (`linux`, `darwin`, `!linux`, etc.)

## Generated Code
- `pkg/version/version.go` -> `go generate` may update `pkg/version/git_version.go`
- `pkg/goDB/protocols/protocols.go` -> generated protocol lookup tables
- `plugins/contrib/contrib_gen.go` -> generated contrib registration
- `pkg/goDB/engine/gen.go` -> benchmark generation hook
- Do not manually edit generated files unless explicitly requested

## Code Style Guidelines

### Imports
- Group imports in standard order: stdlib, first-party, third-party
- First-party imports use `github.com/els0r/goProbe/v4/...`
- Use aliases only when improving clarity or avoiding collisions
- Use side-effect imports (`_`) only for explicit registration behavior

### Formatting and file structure
- Always run `gofmt`; run `goimports` when import lists change
- Keep package names lowercase and concise
- Prefer small focused functions over deep nesting
- Preserve local style in legacy files unless refactoring is requested

### Types and interfaces
- Prefer concrete types unless an interface is required by consumers/tests
- Keep interfaces close to usage sites (example: `query.Runner`)
- Use functional options where pattern already exists in this repo
- Prefer typed constants for flags, statuses, and domain identifiers

### Naming
- Exported identifiers: `PascalCase`; unexported: `camelCase`
- Preserve acronym casing already used (`DB`, `API`, `IP`, `ID`, `URL`)
- Prefer descriptive suffixes (`Config`, `Runner`, `Handler`, `Error`)
- Boolean names should read naturally (`Enabled`, `Is...`, `Has...`, `Disable...`)

### Error handling
- Return errors instead of panicking in production code paths
- Wrap errors with `%w` for context
- Use `errors.Is` / `errors.As` for typed branching
- Keep structured validation errors where established (`huma.ErrorModel`)
- Top-level CLI entrypoints may terminate via fatal logging

### Logging, context, and concurrency
- Use `github.com/els0r/telemetry/logging` for service/runtime logging
- Prefer structured fields (`With("key", value)`)
- Accept `context.Context` as first argument for cancellable/blocking work
- Propagate cancellation and clean up derived contexts (`defer cancel()`)
- Protect shared mutable state with mutexes or confinement

### Testing
- Prefer table-driven tests with clear `t.Run` names
- Use `require` for preconditions and `assert` for value checks
- Keep tests deterministic unless explicitly e2e/integration by design

## PR Title Convention
Validated by `.github/workflows/pr-naming-rules.yml`:
- Regex: `^(\[(feature|bugfix|doc|security|trivial)\])+ [A-Z].+`
- Minimum length: 32
- Maximum length: 256

## Agent Completion Checklist
1. Run `gofmt`/`goimports` on changed Go files
2. Run focused tests for touched packages
3. Run at least one CI-equivalent build/test command with relevant tags
4. Update docs/examples when behavior or flags change
5. If generators are affected, run `go generate` and re-test impacted packages
