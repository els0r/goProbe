# Detail Runs are orchestrated by a second framework-free state machine

Detail Runs and Drill-downs (see CONTEXT.md) live in a plain-TypeScript `DetailRunner` (`src/query/detailRunner.ts`) exposing `open`/`openDrill`/`close` plus `subscribe`/`getSnapshot`, consumed by React through the same `useSyncExternalStore` adapter pattern as the QueryRunner (ADR-0001). One snapshot holds the single panel slot and an optional nested drill slot. The per-kind Query recipes (IP, interface, host, temporal, drill) are pure functions inside the module, deriving from the committed Query and result rows passed at `open()` time. The DetailRunner observes the QueryRunner through a minimal consumer-side interface (`{ subscribe, getSnapshot }`) and closes itself when a new Run starts — the machine, not the caller, enforces "a Detail Run belongs to its Run".

## Considered Options

- **App-level wiring** (App calls `detail.close()` before `runner.run()`) — rejected: the invariant lives in the consumer, so every present and future run-trigger must remember it. That is exactly the manual-enforcement bug class this replaces: five hand-rolled fetch lifecycles with no supersession, where a late response could clobber the open panel.
- **Merging the detail slot into QueryRunner** — rejected: widens a settled, tested module (ADR-0001) with a second concern. The slots genuinely differ — Detail Runs never validate standalone and never stream.
- **Per-panel hooks** — rejected for the same reason ADR-0001 rejected hook-based orchestration: recipes and supersession semantics would only be assertable through a React renderer.

## Consequences

- The DetailRunner owns all AbortControllers; a generation counter makes late events from superseded Detail Runs and Drill-downs inert. Supersession cascades Run → Detail Run → Drill-down: a new Run closes the panel, a new Detail Run discards the drill.
- Recipes always derive from the committed Query of the Run that produced the rows — this deliberately fixes the previous inconsistency where one panel derived from the uncommitted condition draft.
- `hosts_resolver` is always `'string'` for Detail Runs: they address hosts by identifiers the Run already resolved and never re-resolve them.
- Raw `ApiError` is the only error crossing the seam (same rule as ADR-0001). Invalid targets are unrepresentable: targets are typed per kind and `open()` no-ops on empty identifiers, removing the previous synthetic string error.
- `open()` on a target identical to the open panel's target closes it (toggle contract, one targetKey equality inside the module). The selected table row is derived from the snapshot's panel target, not held as separate state.
- The four per-panel `useState` slots, `closeAllDetails`, the Escape juggling in App.tsx, and the six drill-state slices in TemporalRowDetails collapse into the snapshot. The recipes become the repo's second plain-TypeScript test surface.
