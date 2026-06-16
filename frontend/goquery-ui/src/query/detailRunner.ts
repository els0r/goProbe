// DetailRunner — framework-free state machine orchestrating Detail Runs and Drill-downs.
// See CONTEXT.md (Detail Run, Drill-down) and docs/adr/0002-detail-runs-own-state-machine.md.
import { FlowRecord } from '../flows'
import { SummarySchema } from '../api/domain'
import { QueryParamsUI } from './params'
import { ApiError, toApiError } from '../api/errors'
import { buildAttributeQuery, parseAttributeQuery } from '../components/AttributesSelect'

export type DetailPhase = 'loading' | 'done' | 'error'

// What the user selected to inspect. Identifiers are required: open() no-ops on
// empty ones, so an unconstructible target never reaches the seam (ADR-0002).
export type DetailTarget =
  | { kind: 'ip'; ip: string }
  | { kind: 'iface'; hostId: string; iface: string }
  | { kind: 'host'; hostId: string }
  | { kind: 'temporal'; row: FlowRecord }

// Row identity of a temporal Detail Run, used for condition building and display.
export interface TemporalMeta {
  host: string
  host_id: string
  iface: string
  sip: string
  dip: string
  dport?: number | null
  proto?: number | null
}

interface DetailResult {
  phase: DetailPhase
  rows: FlowRecord[]
  summary?: SummarySchema
  // raw ApiError is the only error crossing this seam (same rule as ADR-0001)
  error: ApiError | null
}

export type DetailPanelSnapshot =
  | ({ kind: 'ip'; ip: string } & DetailResult)
  | ({ kind: 'iface'; hostId: string; hostName: string; iface: string } & DetailResult)
  | ({ kind: 'host'; hostId: string; hostName: string } & DetailResult)
  | ({ kind: 'temporal'; row: FlowRecord; meta: TemporalMeta; attrsShown: string[] } & DetailResult)

export type TemporalPanelSnapshot = Extract<DetailPanelSnapshot, { kind: 'temporal' }>

export interface DrillBucket {
  startMs: number
  endMs: number
}

export interface DrillSnapshot extends DetailResult {
  bucket: DrillBucket
  // effective canonical attribute list the Drill-down was executed with
  attrs: string[]
  all: boolean
}

export interface DetailSnapshot {
  panel: DetailPanelSnapshot | null
  drill: DrillSnapshot | null
}

// The committed Query of the Run whose results are being detailed, plus those
// results (host-name lookups and host scoping). Passed at open() time; recipes
// never see draft input (ADR-0002).
export interface DetailBase {
  params: QueryParamsUI
  rows: FlowRecord[]
}

// Consumer-side contract: the minimal client surface a Detail Run needs.
// GlobalQueryClient satisfies it structurally.
export interface DetailQueryClient {
  runQueryUI(
    params: QueryParamsUI,
    signal?: AbortSignal
  ): Promise<{ flows: FlowRecord[]; summary?: SummarySchema }>
}

// Consumer-side contract: the observable face of the QueryRunner. The DetailRunner
// watches it to enforce "a new Run closes the details panel" itself.
export interface RunObservable {
  subscribe(listener: () => void): () => void
  getSnapshot(): { phase: string }
}

const INITIAL: DetailSnapshot = { panel: null, drill: null }

// ---------------------------------------------------------------------------
// Query recipes — pure derivations from (target, base). Exported for tests.
// Every Detail Run addresses hosts by the identifiers its Run already resolved,
// hence hosts_resolver: 'string' (CONTEXT.md, ADR-0002).
// ---------------------------------------------------------------------------

function tupleCondition(meta: TemporalMeta, baseCondition: string | undefined): string {
  const parts: string[] = []
  if (meta.sip) parts.push(`sip=${meta.sip}`)
  if (meta.dip) parts.push(`dip=${meta.dip}`)
  if (meta.dport !== null && meta.dport !== undefined) parts.push(`dport=${meta.dport}`)
  if (meta.proto !== null && meta.proto !== undefined) parts.push(`proto=${meta.proto}`)
  const orig = (baseCondition || '').trim()
  if (orig) parts.push(orig)
  return parts.join(' and ')
}

export function rowMeta(row: FlowRecord): TemporalMeta {
  const hostId = row.host_id || ''
  return {
    host: row.host || hostId,
    host_id: hostId,
    iface: row.iface || '',
    sip: row.sip || '',
    dip: row.dip || '',
    dport: row.dport,
    proto: row.proto,
  }
}

export function deriveDetailQuery(target: DetailTarget, base: DetailBase): QueryParamsUI {
  const p = base.params
  const shared = {
    limit: Math.max(1, p.limit || 1),
    sort_by: 'bytes' as const,
    sort_ascending: false,
    hosts_resolver: 'string',
  }
  switch (target.kind) {
    case 'ip': {
      const baseCond = (p.condition || '').trim()
      const ipCond = `host=${target.ip}`
      // restrict host scoping to hosts that actually carry flows for this IP
      const hostIds = new Set<string>()
      for (const r of base.rows) {
        if ((r.sip === target.ip || r.dip === target.ip) && r.host_id) hostIds.add(r.host_id)
      }
      return {
        ...p,
        ...shared,
        query: 'proto,dport',
        condition: baseCond ? `(${baseCond}) and (${ipCond})` : ipCond,
        query_hosts: hostIds.size ? Array.from(hostIds).join(',') : undefined,
      }
    }
    case 'iface':
      return {
        ...p,
        ...shared,
        query: 'iface,port,protocol',
        query_hosts: target.hostId,
        ifaces: target.iface,
        condition: undefined,
      }
    case 'host':
      return {
        ...p,
        ...shared,
        query: 'iface',
        query_hosts: target.hostId,
        condition: undefined,
      }
    case 'temporal': {
      const meta = rowMeta(target.row)
      return {
        ...p,
        ...shared,
        query: 'time',
        condition: tupleCondition(meta, p.condition) || undefined,
        query_hosts: meta.host_id || undefined,
        ifaces: meta.iface,
        limit: 100000,
      }
    }
  }
}

export function deriveDrillQuery(
  bucket: DrillBucket,
  attrs: { values: string[]; all: boolean },
  meta: TemporalMeta,
  baseParams: QueryParamsUI
): QueryParamsUI {
  return {
    first: new Date(bucket.startMs).toISOString(),
    last: new Date(bucket.endMs).toISOString(),
    ifaces: meta.iface,
    query: buildAttributeQuery(attrs.values, attrs.all),
    query_hosts: meta.host_id || undefined,
    hosts_resolver: 'string',
    condition: tupleCondition(meta, baseParams.condition) || undefined,
    limit: 1000,
    sort_by: 'bytes',
    sort_ascending: false,
  }
}

// ---------------------------------------------------------------------------

function targetKey(t: DetailTarget): string {
  switch (t.kind) {
    case 'ip':
      return `ip:${t.ip}`
    case 'iface':
      return `iface:${t.hostId}|${t.iface}`
    case 'host':
      return `host:${t.hostId}`
    case 'temporal': {
      const r = t.row
      return `temporal:${r.sip}|${r.dip}|${r.dport ?? ''}|${r.proto ?? ''}|${r.host_id ?? ''}|${r.iface ?? ''}`
    }
  }
}

function hasIdentifiers(t: DetailTarget): boolean {
  switch (t.kind) {
    case 'ip':
      return !!t.ip
    case 'iface':
      return !!t.hostId && !!t.iface
    case 'host':
      return !!t.hostId
    case 'temporal':
      return true
  }
}

function hostNameFor(hostId: string, rows: FlowRecord[]): string {
  return rows.find((r) => r.host_id === hostId)?.host || hostId
}

function initialPanel(target: DetailTarget, base: DetailBase): DetailPanelSnapshot {
  const empty: DetailResult = { phase: 'loading', rows: [], error: null }
  switch (target.kind) {
    case 'ip':
      return { kind: 'ip', ip: target.ip, ...empty }
    case 'iface':
      return {
        kind: 'iface',
        hostId: target.hostId,
        hostName: hostNameFor(target.hostId, base.rows),
        iface: target.iface,
        ...empty,
      }
    case 'host':
      return {
        kind: 'host',
        hostId: target.hostId,
        hostName: hostNameFor(target.hostId, base.rows),
        ...empty,
      }
    case 'temporal':
      return {
        kind: 'temporal',
        row: target.row,
        meta: rowMeta(target.row),
        attrsShown: parseAttributeQuery(base.params.query).values,
        ...empty,
      }
  }
}

export class DetailRunner {
  // Snapshot contract: reference-stable between transitions, replaced atomically
  // on each transition (same rule as the QueryRunner snapshot).
  private snapshot: DetailSnapshot = INITIAL
  private listeners = new Set<() => void>()
  // panel and drill supersede independently; close() bumps both
  private panelGen = 0
  private drillGen = 0
  private panelAbort: AbortController | null = null
  private drillAbort: AbortController | null = null
  private panelKey: string | null = null
  private base: DetailBase | null = null
  private lastRunPhase: string | undefined
  private unsubscribeRun: (() => void) | null = null

  constructor(
    private getClient: () => DetailQueryClient,
    run?: RunObservable
  ) {
    if (run) {
      // a Detail Run belongs to the Run whose results it details: when a new Run
      // starts (phase enters 'validating'), the panel closes — enforced here, not
      // by the caller
      this.lastRunPhase = run.getSnapshot().phase
      this.unsubscribeRun = run.subscribe(() => {
        const phase = run.getSnapshot().phase
        const runStarted = phase === 'validating' && this.lastRunPhase !== 'validating'
        this.lastRunPhase = phase
        if (runStarted) this.close()
      })
    }
  }

  getSnapshot(): DetailSnapshot {
    return this.snapshot
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  // Starts a Detail Run for the target, superseding panel and drill. Opening the
  // target that is already open closes it instead (toggle contract). Targets with
  // empty identifiers are ignored.
  open(target: DetailTarget, base: DetailBase): void {
    if (!hasIdentifiers(target)) return
    const key = targetKey(target)
    if (this.snapshot.panel && this.panelKey === key) {
      this.close()
      return
    }
    const gen = ++this.panelGen
    this.drillGen++
    this.abortInflight()
    this.base = base
    this.panelKey = key
    this.transition({ panel: initialPanel(target, base), drill: null })
    void this.fetchPanel(deriveDetailQuery(target, base), gen)
  }

  // Starts a Drill-down into one time bucket of the open temporal panel,
  // superseding any Drill-down in flight. No-ops unless a temporal panel is open.
  openDrill(bucket: DrillBucket, attrs: { values: string[]; all: boolean }): void {
    const panel = this.snapshot.panel
    if (!panel || panel.kind !== 'temporal' || !this.base) return
    if (!attrs.all && attrs.values.length === 0) return
    const gen = ++this.drillGen
    this.drillAbort?.abort()
    this.drillAbort = null
    const params = deriveDrillQuery(bucket, attrs, panel.meta, this.base.params)
    // store the effective canonical attribute list the results will carry
    const effective = parseAttributeQuery(params.query)
    this.transition({
      drill: {
        bucket,
        attrs: effective.values,
        all: attrs.all,
        phase: 'loading',
        rows: [],
        error: null,
      },
    })
    void this.fetchDrill(params, gen)
  }

  // Discards the Drill-down (e.g. when another bucket is selected) without
  // touching the panel.
  closeDrill(): void {
    this.drillGen++
    this.drillAbort?.abort()
    this.drillAbort = null
    if (this.snapshot.drill) this.transition({ drill: null })
  }

  // Closes the panel; the Drill-down is discarded with it (supersession cascades).
  close(): void {
    this.panelGen++
    this.drillGen++
    this.abortInflight()
    this.base = null
    this.panelKey = null
    if (this.snapshot.panel || this.snapshot.drill) this.transition({ panel: null, drill: null })
  }

  // Detaches from the observed QueryRunner (component unmount / test teardown).
  dispose(): void {
    this.unsubscribeRun?.()
    this.unsubscribeRun = null
    this.close()
  }

  private transition(patch: Partial<DetailSnapshot>) {
    this.snapshot = { ...this.snapshot, ...patch }
    for (const l of [...this.listeners]) l()
  }

  private abortInflight() {
    this.panelAbort?.abort()
    this.panelAbort = null
    this.drillAbort?.abort()
    this.drillAbort = null
  }

  private async fetchPanel(params: QueryParamsUI, gen: number) {
    const ctrl = new AbortController()
    this.panelAbort = ctrl
    try {
      const data = await this.getClient().runQueryUI(params, ctrl.signal)
      if (gen !== this.panelGen) return
      this.panelAbort = null
      this.transition({
        panel: { ...this.snapshot.panel!, phase: 'done', rows: data.flows, summary: data.summary },
      })
    } catch (e) {
      if (gen !== this.panelGen) return
      this.panelAbort = null
      this.transition({ panel: { ...this.snapshot.panel!, phase: 'error', error: toApiError(e) } })
    }
  }

  private async fetchDrill(params: QueryParamsUI, gen: number) {
    const ctrl = new AbortController()
    this.drillAbort = ctrl
    try {
      const data = await this.getClient().runQueryUI(params, ctrl.signal)
      if (gen !== this.drillGen) return
      this.drillAbort = null
      this.transition({
        drill: { ...this.snapshot.drill!, phase: 'done', rows: data.flows, summary: data.summary },
      })
    } catch (e) {
      if (gen !== this.drillGen) return
      this.drillAbort = null
      this.transition({ drill: { ...this.snapshot.drill!, phase: 'error', error: toApiError(e) } })
    }
  }
}
