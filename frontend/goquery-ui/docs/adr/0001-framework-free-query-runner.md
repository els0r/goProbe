# Query runs are orchestrated by a framework-free QueryRunner state machine

Query orchestration (preflight validation, streaming/non-streaming execution, supersession, cancellation, per-host error bookkeeping) lives in a plain-TypeScript `QueryRunner` (`src/query/runner.ts`) exposing `run`/`validate`/`cancel` plus `subscribe`/`getSnapshot`, consumed by React through a thin `useSyncExternalStore` adapter. We chose this over the obvious alternatives because the runner's Module Interface is the codebase's first test surface — it must be drivable without React — and because the domain has exactly one result slot: a new Run supersedes the one in flight.

## Considered Options

- **TanStack Query / SWR** — rejected: their request/response cache model fits neither SSE partial results nor supersession semantics (one mutable result slot, not keyed cache entries), and the project deliberately carries no state-management dependency.
- **Hook-based orchestration (`useQuery`-style custom hook)** — rejected: ties the only testable behaviour surface to a React renderer; with zero test infrastructure in the repo, the state machine must be assertable as plain TypeScript.
- **Callback-prop wiring (status quo, one level up)** — rejected: reproduces the 12-useState-slice problem; per-event callbacks force the consumer to reassemble state that is the machine's job to hold.

## Consequences

- The runner owns all AbortControllers and stream closers; callers only ever `cancel()`. A run-generation counter makes late events from superseded Runs inert.
- Connection-level stream failure is fatal (`phase: 'error'`); per-host failures are non-fatal Host Errors. This deliberately fixes a latent bug where a dead SSE connection left the UI loading forever.
- Snapshots are reference-stable between transitions and replaced atomically — required by `useSyncExternalStore`, and the reason `cancel()` can transition synchronously (which obsoletes the 120ms post-cancel cooldown previously in App.tsx).
- Raw `ApiError` crosses the seam; field-level mapping (`mapValidationError`) and run-start policy remain App concerns. The run-start policy stated here ("auto-run on param change, streaming opt-in") is **superseded by ADR-0005**: auto-run is now debounced and scoped to structured inputs across both modes, while the mechanism (AbortControllers, generation counter, atomic snapshots) stays as described above.
