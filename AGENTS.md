# AGENTS.md

## Purpose
This guide defines repository-specific instructions for coding agents working in this project.
Use it to keep changes aligned with existing code style and CI behavior.

## Operating Defaults
- Be direct and concise. Avoid filler.
- If something is wrong, say so and propose a fix. If no safe fix exists yet, state that clearly.
- For non-trivial code changes, state a short approach before implementing.
- Do not guess repository structure, behavior, or conventions. Inspect relevant files first.
- Primary implementation languages in this repo are Go and Bash. Apply language-appropriate idioms and tooling.

## Instruction and Reference Order
When task-specific direction is missing, use this lookup order:
1. This file (`AGENTS.md`)
2. Task-relevant docs in the repository, especially:
   - `README.md`
   - `cmd/goProbe/README.md`
   - `cmd/goQuery/README.md`
   - `cmd/gpdb/README.md`
   - `pkg/query/README.md`
   - `examples/README.md`
3. CI workflows and release pipelines as source of truth for verification and packaging behavior:
   - `.github/workflows/ci-pr.yml`
   - `.github/workflows/ci-push.yml`
   - `.github/workflows/build-packages.yml`
4. Existing patterns in nearby code

## Project Layout
- Language: Go (`go 1.25`)
- Module: `github.com/els0r/goProbe/v4`
- Workspace: `go.work` includes `.` and `./plugins/contrib`
- Primary binaries:
  - `./cmd/goProbe`
  - `./cmd/goQuery`
  - `./cmd/global-query`
  - `./cmd/gpctl`
  - `./cmd/gpdb`
- Primary code folders:
  - `pkg/` (API, capture, query, goDB, results, shared types)
  - `plugins/` (resolver/querier plugin registration)

## Build Commands
Run commands from repository root.

### CI-equivalent builds
- `GOOS=linux GOARCH=amd64 go build -tags jsoniter -v ./...`
- `GOOS=linux GOARCH=amd64 go build -tags jsoniter,slimcap_nomock -v ./...`
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags jsoniter -v ./...`

### Build individual binaries
- `go build -tags jsoniter -o goProbe ./cmd/goProbe`
- `go build -tags jsoniter -o goQuery ./cmd/goQuery`
- `go build -tags jsoniter -o global-query ./cmd/global-query`
- `go build -tags jsoniter -o gpctl ./cmd/gpctl`
- `go build -tags jsoniter -o gpdb ./cmd/gpdb`

### Release-like builds (workflow parity)
- `GOOS=linux GOARCH=amd64 go build -a -tags jsoniter,slimcap_nomock -pgo=auto -o goProbe ./cmd/goProbe`
- `GOOS=linux GOARCH=amd64 go build -a -tags jsoniter -pgo=auto -o global-query ./cmd/global-query`
- `GOOS=linux GOARCH=amd64 go build -a -pgo=auto -o goQuery ./cmd/goQuery`
- `GOOS=linux GOARCH=amd64 go build -a -pgo=auto -o gpctl ./cmd/gpctl`
- `GOOS=linux GOARCH=amd64 go build -a -o gpdb ./cmd/gpdb`

## Test Commands

### Full and CI variants
- `go test -tags jsoniter -v ./...`
- `go test -tags jsoniter,slimcap_nomock -v ./...`
- `CGO_ENABLED=0 go test -tags jsoniter -v ./...`
- `go test -tags jsoniter -race -v ./...`

### Push workflow variant
- `go test -tags jsoniter -skip-benchmarks -v ./...`

### Package-level testing
- `go test -tags jsoniter -v ./pkg/query`
- `go test -tags jsoniter -v ./pkg/goDB/engine`

### Run a single test
- `go test -tags jsoniter -run '^TestPrepareArgs$' -v ./pkg/query`

### Run a single subtest
- `go test -tags jsoniter -run '^TestPrepareArgs/valid query args$' -v ./pkg/query`

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

## Design and Code Style Guidelines

### System design principles
- Prefer KISS solutions. Add complexity only when it clearly improves correctness, maintainability, or performance.
- Favor clear control flow. Use guard clauses and early returns to avoid deep nesting.
- Keep cyclomatic complexity low. Flatten branching and extract focused helpers.
- Apply DRY with judgment. Reduce duplication, but avoid introducing unnecessary abstractions or dependencies.
- Prefer small focused files and functions; avoid growing already large files when extraction is practical.

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
- Never ignore returned errors
- Return errors instead of panicking in production code paths
- Wrap errors with `%w` for context
- Use `errors.Is` / `errors.As` for typed branching
- Keep structured validation errors where established (`huma.ErrorModel`)
- Top-level CLI entrypoints may terminate via fatal logging

### Logging, context, and concurrency
- Use `github.com/els0r/telemetry/logging` for service/runtime logging
- Prefer structured fields (`Info("log message", "key1", value1, "key2", value2, ...)`)
- Accept `context.Context` as first argument for cancellable/blocking work
- Propagate cancellation and clean up derived contexts (`defer cancel()`)
- Protect shared mutable state with mutexes or confinement

### Testing
- Prefer table-driven tests with clear `t.Run` names
- Use `require` for preconditions and `assert` for value checks
- Keep tests deterministic unless explicitly e2e/integration by design

## Dependency Policy
- Do not add dependencies without stating why existing stdlib/current dependencies are insufficient.
- Keep dependency changes minimal and task-scoped.
- Avoid new dependencies for simple convenience wrappers.

## Generated Code
- `pkg/version/version.go` -> `go generate` may update `pkg/version/git_version.go`
- `pkg/goDB/protocols/protocols.go` -> generated protocol lookup tables
- `plugins/contrib/contrib_gen.go` -> generated contrib registration
- `pkg/goDB/engine/gen.go` -> benchmark generation hook
- Do not manually edit generated files unless explicitly requested

## Do Not
- Do not generate code that ignores error returns.
- Do not add dependencies without documenting rationale.
- Do not guess project structure; inspect the repository first.
- Do not manually edit generated files unless explicitly requested.
- Do not add boilerplate explanations for standard library behavior unless the task asks for explanation.

## PR Title Convention
Validated by `.github/workflows/pr-naming-rules.yml`:
- Regex: `^(\[(feature|bugfix|doc|security|trivial)\])+ [A-Z].+`
- Minimum length: 32
- Maximum length: 256

## Agent Completion Checklist
1. For non-trivial changes, state a brief approach before implementation.
2. Run `gofmt`/`goimports` on changed Go files.
3. Run focused tests for touched packages.
4. Run at least one CI-equivalent build/test command with relevant tags.
5. Update docs/examples when behavior, APIs, or flags change.
6. If generators are affected, run `go generate` and re-test impacted packages.
7. Report exactly which verification commands were run.
