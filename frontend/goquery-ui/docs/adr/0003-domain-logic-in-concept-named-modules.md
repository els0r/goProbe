# Domain logic lives in concept-named modules, not `utils/`

Logic that operates on a domain type lives in a module named for the concept it owns, not in a catch-all `utils/` package and not under the transport-named `api/`. The first such module is `src/flows/` (`record.ts`, `totals.ts`, `group.ts`, re-exported from `index.ts`), which owns the **Flow** concept: `FlowRecord` + `flattenRow`/`extractFlows`, the **Run Total** (`ResultTotals` + `resultTotals`), the per-Flow total, **Run Share** (`runSharePct`), and the grouping/scale helpers (`sumTotals`, `groupByService`, `groupByIface`, `inOutScaleMax`) previously misfiled under `utils/aggregation.ts` and `utils/inOutBar.ts`. The per-Flow `bytes_total`/`packets_total` are derived once in `flattenRow` as record fields — the same boundary, and same pattern, as the existing `bidirectional` field. `ApiError` moves into `api/errors.ts` so `api/` becomes purely transport. `utils/` is left holding only genuinely domain-free helpers (`format.ts`, the diverging-bar sqrt geometry in `inOutBar.ts`).

## Considered Options

- **Grow `utils/aggregation.ts`** (status quo) — rejected: `utils/` is a catch-all with no contract, only contents. It already conflated Flow operations, network helpers (`proto`, `ipClassify`), time formatting, and query sanitization. Adding the row total and Run Share here deepens the ball of mud rather than naming the concept.
- **Keep domain logic in `api/domain.ts`** — rejected: cements domain logic under a folder named for transport (`client.ts`, `sse.ts`, `generated.ts`). The anchor (`FlowRecord`, `ResultTotals`) already lived there only because there was nowhere else; the fix is to give the concept a home, not to enshrine the accident.
- **Layer-named `src/domain/`** — rejected: a folder named by layer ("domain") drifts back into a catch-all as every non-Flow domain type (`QueryParamsUI`, `ApiError`) gets pulled in. `flows/` is named by the concept it owns, so its boundary is self-policing.
- **A `recordTotals(r)` helper instead of derived fields** — rejected: the per-Flow total is a derivation of the in/out counters, exactly like `bidirectional`, which `flattenRow` already computes at the boundary. A helper would be a second derivation path diverging from the established one, and would leave the ~20 `bytes_in + bytes_out` call sites each carrying an import and a call instead of reading a field.
- **Big-bang `utils/` teardown** — rejected for now: dissolving every cluster (`net/`, `time/`, `query/`) in one change risks recreating the problem by lumping concepts into fresh homes nobody reasoned about. Flows is extracted first as a tracer bullet; each remaining cluster is its own home decision and its own change, citing this ADR.

## Consequences

- `bytes_in + bytes_out` (and the packets twin), re-derived at ~20 sites across `exportText.ts`, `TableView.tsx`, `TemporalRowDetails.tsx`, and `GraphView.tsx`, collapse to reading `r.bytes_total` / `r.packets_total`. The row total is computed once, at the transport boundary, named identically to the `ResultTotals` aggregate.
- `runSharePct(part, runTotal)` owns the clamp-to-`[0,100]` and the `runTotal > 0` guard. It unifies only the Run-Share sites (table + export); the temporal heatmap's peak-bucket ratio and the diverging-bar sqrt geometry are deliberately *not* folded in — they are not shares of the Run (see CONTEXT.md).
- Display precision stays at the call site as a deliberate per-surface choice: 1 decimal in the space-constrained table, 2 in the text export. The helper returns a `number`; formatting is presentation.
- `QueryParamsUI` and `PagedLike` stay in `api/domain.ts` for now, flagged as a future `query/` domain home. This ADR does not move them; it sets the precedent they will follow.
- The remaining `utils/` clusters — `proto.ts`/`ipClassify.ts` (a network domain), the `temporal.ts`/`timeFormat.ts`/`timeRange.ts` trio (time), `inputSanitize.ts` (query), `errorMapping.ts`/`renderError.ts` (error presentation) — are now explicitly out-of-place and queued for the same treatment, each in its own change.

## Realized — `query/` home

The `query/` home flagged above is now established as a **full concept module**, owning the **Query** end to end:

- **Shape + hygiene** — `QueryParamsUI` moves `api/domain.ts` → `query/params.ts`, joined by `sanitizeUIParams`/`sanitizeHostList` and the canonical `DEFAULTS` (the query half of the now-dissolved `utils/inputSanitize.ts`). `api/domain.ts` is left holding only generated-schema aliases — pure transport, as this ADR intended for `api/`.
- **Serialization** — `state/queryState.ts` (`serializeParams`/`parseParams`) moves to `query/serialize.ts`, and the layer-named `state/` folder is dissolved. A Query's URL form belongs to the **Query** concept, not to a layer — the same concept-over-layer reasoning this ADR used to reject `src/domain/`, now applied to `state/`.
- **Public surface** — `query/index.ts` re-exports the concept (`params`, `serialize`, `runner`, `detailRunner`, `autoRun` + hooks), mirroring the `flows/` barrel.
- `PagedLike` was unused at every call site and is **deleted**, not moved.
- The error-presentation half of `inputSanitize.ts` (`formatValue`/`normalizeText`, consumed by `errorMapping.ts` and `ErrorBanner.tsx`) is **not** Query-domain; it is renamed in place to `utils/errorText.ts` and awaits the error-presentation home (the `errorMapping.ts`/`renderError.ts` cluster) still queued above.
