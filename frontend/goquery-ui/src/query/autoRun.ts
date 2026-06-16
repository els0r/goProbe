// autoRun.ts — policy for when a Query Run fires on its own (ADR-0005).
// Pure functions only; the run *mechanism* stays in the QueryRunner (ADR-0001).
import { QueryParamsUI } from './params'
import { RunPhase } from './runner'

// structuralKey is the identity of the *structured* parameters whose change
// auto-runs a Query: time range, attributes (encoded in `query`), sort, and
// limit. The three free-text fields (query_hosts, ifaces, condition) are
// deliberately excluded — they commit explicitly, never via auto-run. A change
// to this key is the auto-run trigger; a free-text edit leaves it untouched.
export function structuralKey(p: QueryParamsUI): string {
  return JSON.stringify([p.first, p.last, p.query, p.sort_by, p.sort_ascending, p.limit])
}

// Free-text fields commit explicitly, never via auto-run. isDirty reports
// whether the current free-text params differ from the snapshot taken when the
// last Run started — i.e. there are edits the user has not yet Run. Only the
// three free-text fields are compared; empty string and undefined are equal.
type FreeTextFields = Pick<QueryParamsUI, 'query_hosts' | 'ifaces' | 'condition'>

export function isDirty(current: FreeTextFields, lastRun: FreeTextFields | null): boolean {
  if (!lastRun) return false
  const norm = (v?: string) => (v ?? '').trim()
  return (
    norm(current.query_hosts) !== norm(lastRun.query_hosts) ||
    norm(current.ifaces) !== norm(lastRun.ifaces) ||
    norm(current.condition) !== norm(lastRun.condition)
  )
}

export type RunButtonState = 'refresh' | 'apply' | 'busy' | 'cancel'

// The single Run control's state, from run phase + dirty + how long the current
// Run has been in flight. While running it shows a disabled, spinning "Run"
// (busy) until elapsedMs crosses the threshold, then morphs to Cancel — so fast
// auto-runs never flicker Cancel. Idle, it is Apply when there are unapplied
// free-text edits, else Refresh. (ADR-0005)
export function runButtonState(input: {
  phase: RunPhase
  dirty: boolean
  elapsedMs: number
  threshold: number
}): RunButtonState {
  const { phase, dirty, elapsedMs, threshold } = input
  const running = phase === 'validating' || phase === 'running'
  if (running) return elapsedMs >= threshold ? 'cancel' : 'busy'
  return dirty ? 'apply' : 'refresh'
}
