import React, { useEffect, useState, useCallback, useRef } from 'react'
import { getGlobalQueryClient, setGlobalQueryBaseUrl } from '../api/client'
import { FlowRecord, QueryParamsUI, SummarySchema } from '../api/domain'
import { parseParams, serializeParams } from '../state/queryState'
import { TableView } from '../views/TableView'
import { GraphView } from '../views/GraphView'
import { IpDetailsPanel } from '../views/IpDetailsPanel'
import { IfaceDetailsPanel } from '../views/IfaceDetailsPanel'
import { HostDetailsPanel } from '../views/HostDetailsPanel'
import { TemporalDetailsPanel } from '../views/TemporalDetailsPanel'
import { AttributesSelect, parseAttributeQuery, buildAttributeQuery } from './AttributesSelect'
import { buildTextTable } from '../views/exportText'
import { env } from '../env'
import { formatDurationNs, humanBytes, humanPackets } from '../utils/format'
import { DisplaySummary } from './DisplaySummary'

interface ErrorBannerProps {
  error: unknown
}
function ErrorBanner({ error }: ErrorBannerProps) {
  if (!error) return null
  const [open, setOpen] = React.useState(false)
  let simple = ''
  let problem: any | undefined
  if (typeof error === 'string') simple = error
  else if (error && typeof error === 'object') {
    const e: any = error
    simple = e.message || 'error'
    if (e.problem) problem = e.problem
  }
  return (
    <div className="mb-3 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm">
      <div className="flex items-center justify-between">
        <div className="font-medium text-red-300">{simple}</div>
        {problem && (
          <button
            type="button"
            onClick={() => setOpen((o) => !o)}
            className="text-[11px] rounded px-2 py-0.5 text-red-300 hover:text-white hover:bg-red-500/20 ring-1 ring-red-500/30"
          >
            {open ? 'Hide details' : 'Show details'}
          </button>
        )}
      </div>
      {problem && open && (
        <div className="mt-2 space-y-1 text-red-200/90">
          {problem.detail && <div className="text-[12px]">{problem.detail}</div>}
          {Array.isArray(problem.errors) && problem.errors.length > 0 && (
            <ul className="mt-1 max-h-60 overflow-auto rounded bg-red-500/5 p-2 text-[11px] leading-snug ring-1 ring-red-500/20">
              {problem.errors.map((er: any, i: number) => (
                <li key={i} className="mb-2 last:mb-0">
                  <div>
                    <span className="font-mono text-red-300">{er.location || '(unknown)'}:</span>{' '}
                    {er.message || 'validation error'}
                  </div>
                  {er.value !== undefined &&
                    (typeof er.value === 'object' && er.value !== null ? (
                      <pre className="mt-1 max-h-40 overflow-auto whitespace-pre rounded bg-black/30 p-2 text-[11px] text-red-200/90 ring-1 ring-red-500/20">
                        {JSON.stringify(er.value, null, 2)}
                      </pre>
                    ) : (
                      <div className="opacity-70">value: {formatValue(er.value)}</div>
                    ))}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}

function formatValue(v: unknown): string {
  if (v === null) return 'null'
  if (v === undefined) return 'undefined'
  if (typeof v === 'string') return JSON.stringify(v)
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try {
    return JSON.stringify(v)
  } catch {
    return '[unserializable]'
  }
}

// normalize strings for error matching: lower-case and unify curly apostrophes
function normalizeText(s: string | undefined | null): string {
  if (!s) return ''
  return String(s)
    .toLowerCase()
    .replace(/\u2019/g, "'")
    .trim()
}

function formatTimestamp(ts: string | undefined): string {
  if (!ts) return '—'
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ts
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  } catch {
    return ts
  }
}

function humanRangeDuration(startIso?: string | null, endIso?: string | null): string {
  if (!startIso || !endIso) return ''
  const start = new Date(startIso).getTime()
  const end = new Date(endIso).getTime()
  if (!isFinite(start) || !isFinite(end)) return ''
  let ms = Math.max(0, end - start)
  const dayMs = 24 * 60 * 60 * 1000
  const hourMs = 60 * 60 * 1000
  const minMs = 60 * 1000
  const secMs = 1000
  const d = Math.floor(ms / dayMs)
  ms -= d * dayMs
  const h = Math.floor(ms / hourMs)
  ms -= h * hourMs
  const m = Math.floor(ms / minMs)
  ms -= m * minMs
  const s = Math.floor(ms / secMs)
  const parts: string[] = []
  if (d > 0) parts.push(d + 'd')
  if (h > 0 || d > 0) parts.push(h + 'h')
  if (m > 0 || (d === 0 && h === 0)) parts.push(m + 'm')
  // Only show seconds if duration < 1m
  if (d === 0 && h === 0 && m === 0) parts.push(s + 's')
  return parts.join('')
}

function isoToLocalInput(iso?: string | null): string {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return ''
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return ''
  }
}

function localInputToIso(val: string): string | undefined {
  if (!val) return undefined
  const d = new Date(val)
  if (isNaN(d.getTime())) return undefined
  return d.toISOString()
}

// sanitize comma-separated host list: trim items and drop empties
function sanitizeHostList(raw?: string | null): string | undefined {
  if (!raw) return undefined
  const items = String(raw)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  return items.length ? items.join(',') : undefined
}

interface SummaryStatProps {
  label: string
  value: React.ReactNode
  multiline?: boolean
}
function SummaryStat({ label, value, multiline }: SummaryStatProps) {
  const isSimple = typeof value === 'string' || typeof value === 'number'
  const valueClass = multiline
    ? 'text-[13px] font-medium text-gray-100 leading-tight break-words'
    : 'truncate text-[13px] font-medium text-gray-100'
  return (
    <div className="flex flex-col rounded-md bg-surface-200/40 px-2 py-2 ring-1 ring-white/5">
      <div className="mb-0.5 text-[10px] uppercase tracking-wide text-gray-400">{label}</div>
      <div className={valueClass} title={isSimple ? String(value) : undefined}>
        {value}
      </div>
    </div>
  )
}

const DEFAULT_FIRST_MINUTES = 10
const DEFAULTS: QueryParamsUI = {
  first: '',
  last: '',
  ifaces: '',
  query: '',
  condition: undefined as any,
  limit: 200,
  sort_by: 'bytes',
  sort_ascending: false,
}

// Ensure any externally loaded params (e.g., from localStorage) are valid
function sanitizeUIParams(p: any): QueryParamsUI {
  const merged: QueryParamsUI = {
    ...DEFAULTS,
    ...(p || {}),
  }
  merged.sort_by = merged.sort_by === 'packets' ? 'packets' : 'bytes'
  merged.sort_ascending = !!merged.sort_ascending
  merged.limit = Math.max(1, Number(merged.limit) || DEFAULTS.limit)
  merged.first = typeof merged.first === 'string' ? merged.first : DEFAULTS.first
  merged.last = typeof merged.last === 'string' ? merged.last : DEFAULTS.last
  merged.ifaces = typeof merged.ifaces === 'string' ? merged.ifaces : ''
  merged.query = typeof merged.query === 'string' ? merged.query : ''
  return merged
}

function buildInitialParams(): QueryParamsUI {
  const parsed = parseParams(window.location.search)
  const lastDate = new Date()
  const firstDate = new Date(lastDate.getTime() - DEFAULT_FIRST_MINUTES * 60 * 1000)
  // normalize attributes selection: if empty/All, store explicit list to satisfy backend min-length
  const attr = parseAttributeQuery(parsed.query)
  const normalizedQuery = buildAttributeQuery(attr.values, attr.all)
  return {
    ...DEFAULTS,
    ...parsed,
    query_hosts: sanitizeHostList(parsed.query_hosts),
    query: normalizedQuery,
    first: parsed.first || firstDate.toISOString(),
    last: parsed.last || lastDate.toISOString(),
  }
}

const tabs = [
  { id: 'table', label: 'Table' },
  { id: 'graph', label: 'Graph' },
] as const

type TabId = (typeof tabs)[number]['id']

export default function App() {
  const [params, setParams] = useState<QueryParamsUI>(() => buildInitialParams())
  const [activeTab, setActiveTab] = useState<TabId>('table')
  const [rows, setRows] = useState<FlowRecord[]>([])
  const [summary, setSummary] = useState<SummarySchema | undefined>(undefined)
  const [ipDetail, setIpDetail] = useState<{
    ip: string
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
  } | null>(null)
  const [ifaceDetail, setIfaceDetail] = useState<{
    host: string
    iface: string
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
  } | null>(null)
  const [hostDetail, setHostDetail] = useState<{
    hostId: string
    hostName: string
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
  } | null>(null)
  const [temporalDetail, setTemporalDetail] = useState<{
    meta: {
      host: string
      iface: string
      sip: string
      dip: string
      dport?: number | null
      proto?: number | null
    }
    attrsShown: string[]
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
  } | null>(null)
  const [loading, setLoading] = useState(false)
  const [streaming, setStreaming] = useState(false)
  const [error, setError] = useState<unknown>('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [streamErrors, setStreamErrors] = useState<Array<{ message?: string; host?: string }>>([])
  const [progress, setProgress] = useState<{ done?: number; total?: number }>({})
  const [hostsStatuses, setHostsStatuses] = useState<
    Record<string, { code?: string; message?: string }>
  >({})
  const [hostErrorCount, setHostErrorCount] = useState<number>(0)
  const [hostOkCount, setHostOkCount] = useState<number>(0)
  const [ifaceDetailOpen, setIfaceDetailOpen] = useState<boolean>(false)
  const [copiedToast, setCopiedToast] = useState<boolean>(false)
  const streamCloserRef = useRef<{ close: () => void } | null>(null)
  // settings state
  const defaultBackend = env.GQ_API_BASE_URL || 'http://localhost:8145'
  const LS_BACKEND_KEY = 'goquery_ui_backend_url'
  const LS_STREAMING_KEY = 'goquery_ui_use_streaming'
  const LS_HOSTS_RESOLVER_KEY = 'goquery_ui_hosts_resolver'
  const [backendUrl, setBackendUrl] = useState<string>(() => {
    try {
      const saved = localStorage.getItem(LS_BACKEND_KEY)
      return saved || defaultBackend
    } catch {
      return defaultBackend
    }
  })
  const [useStreaming, setUseStreaming] = useState<boolean>(() => {
    try {
      const saved = localStorage.getItem(LS_STREAMING_KEY)
      if (saved === '1' || saved === 'true') return true
      if (saved === '0' || saved === 'false') return false
    } catch {}
    // fallback to runtime env default
    return !!env.SSE_ON_LOAD
  })
  // Hosts Resolver selection; prefer a saved value if valid, else first available option
  const [hostsResolver, setHostsResolver] = useState<string>(() => {
    const opts = Array.isArray(env.HOST_RESOLVER_TYPES) ? env.HOST_RESOLVER_TYPES : []
    try {
      const saved = localStorage.getItem(LS_HOSTS_RESOLVER_KEY)
      if (typeof saved === 'string' && saved.length > 0 && opts.includes(saved)) return saved
    } catch {}
    return opts[0] || ''
  })
  const [settingsOpen, setSettingsOpen] = useState<boolean>(false)

  // persist backend selection and apply to client on change
  useEffect(() => {
    try {
      localStorage.setItem(LS_BACKEND_KEY, backendUrl)
    } catch {}
    setGlobalQueryBaseUrl(backendUrl)
  }, [backendUrl])

  // persist streaming preference
  useEffect(() => {
    try {
      localStorage.setItem(LS_STREAMING_KEY, useStreaming ? '1' : '0')
    } catch {}
  }, [useStreaming])

  // persist hosts resolver selection
  useEffect(() => {
    try {
      localStorage.setItem(LS_HOSTS_RESOLVER_KEY, hostsResolver)
    } catch {}
  }, [hostsResolver])

  // guard against stale/invalid saved value not present in env options
  useEffect(() => {
    const opts = Array.isArray(env.HOST_RESOLVER_TYPES) ? env.HOST_RESOLVER_TYPES : []
    if (hostsResolver && !opts.includes(hostsResolver)) {
      const fallback = opts[0] || ''
      if (fallback !== hostsResolver) setHostsResolver(fallback)
    }
  }, [hostsResolver])

  // global Escape closes any details, settings, or interfaces modal
  useEffect(() => {
    const onEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        if (ifaceDetailOpen) {
          setIfaceDetailOpen(false)
          e.preventDefault()
          return
        }
        if (ipDetail || ifaceDetail || hostDetail || temporalDetail) {
          closeAllDetails()
          e.preventDefault()
          return
        }
        if (settingsOpen) {
          setSettingsOpen(false)
          e.preventDefault()
        }
      }
    }
    window.addEventListener('keydown', onEsc)
    return () => window.removeEventListener('keydown', onEsc)
  }, [ipDetail, ifaceDetail, hostDetail, temporalDetail, settingsOpen, ifaceDetailOpen])
  // interfaces free-text (comma separated). only commit on blur / enter to reduce churn
  const [ifacesInput, setIfacesInput] = useState(params.ifaces)
  // hosts free text field stored in query_hosts
  const [hostsInput, setHostsInput] = useState(params.query_hosts || '')
  // condition buffered input (commit only on space or blur)
  const [conditionInput, setConditionInput] = useState(params.condition || '')
  // attributes multi-select encoded into params.query (empty string => All)
  const attrState = parseAttributeQuery(params.query)
  function onAttributesChange(next: { values: string[]; all: boolean }) {
    const q = buildAttributeQuery(next.values, next.all)
    setParams((p) => ({ ...p, query: q }))
  }
  function closeAllDetails() {
    setIpDetail(null)
    setIfaceDetail(null)
    setHostDetail(null)
    setTemporalDetail(null)
  }
  function commitInterfaces() {
    setParams((p) => ({ ...p, ifaces: ifacesInput.trim() }))
  }
  function commitHosts() {
    setParams((p) => ({ ...p, query_hosts: sanitizeHostList(hostsInput) }))
  }
  function commitCondition(next?: string) {
    const raw = next ?? conditionInput
    setParams((p) => ({
      ...p,
      condition: raw.trim() ? raw : undefined,
    }))
    // Trigger validation immediately when committing from SPACE/blur
    void validateCurrent({ conditionOverride: raw })
  }

  // --- Validation helpers ---
  const validateAbortRef = useRef<AbortController | null>(null)
  // Abort controller for non-streaming runs
  const runAbortRef = useRef<AbortController | null>(null)
  // Small cooldown to avoid racing a new request onto a just-aborted connection
  const lastCancelAtRef = useRef<number>(0)

  // Map API error to field errors and a banner error, mirroring run() behavior
  const mapValidationError = useCallback(
    (e: any): { fields: Record<string, string>; banner: any } => {
      // Ignore user-initiated aborts: don't surface as banner errors
      try {
        const name = String((e as any)?.name || '')
        const msg = String((e as any)?.message || '')
        if (name === 'AbortError' || normalizeText(msg) === 'request aborted') {
          return { fields: {}, banner: '' }
        }
      } catch {}
      const fields: Record<string, string> = {}
      let banner: any = e
      if (
        e &&
        typeof e === 'object' &&
        (e as any).problem &&
        Array.isArray((e as any).problem.errors)
      ) {
        const isLoc = (loc: string, key: string) =>
          loc === `body.${key}` ||
          loc.startsWith(`body.${key}.`) ||
          loc.startsWith(`body.${key}[`) ||
          loc === key
        for (const er of (e as any).problem.errors as any[]) {
          const locRaw = String(er.location || '')
          const loc = locRaw.toLowerCase()
          const rawMsg = String(er.message || 'validation error').trim()
          const isCondition = isLoc(loc, 'condition')
          let msg = isCondition
            ? rawMsg.replace(
                /^(\s*)([a-z])/,
                (_m: string, ws: string, ch: string) => ws + ch.toUpperCase()
              )
            : rawMsg.charAt(0).toUpperCase() + rawMsg.slice(1)
          if (!isCondition && er.value !== undefined) msg += ` -- value: ${formatValue(er.value)}`
          const normRaw = normalizeText(rawMsg)
          if (
            normRaw === 'list of target hosts is empty' ||
            normRaw === "couldn't prepare query: list of target hosts is empty" ||
            normRaw.includes('list of target hosts is empty')
          ) {
            fields.hosts = msg
            continue
          }
          const isResolverField =
            isLoc(loc, 'query_hosts_resolver_type') || isLoc(loc, 'hosts_resolver')
          const isHostsField =
            isLoc(loc, 'query_hosts') || isLoc(loc, 'hostname') || isLoc(loc, 'host_id')
          if (isLoc(loc, 'ifaces')) fields.ifaces = msg
          else if (!isResolverField && isHostsField) fields.hosts = msg
          else if (isLoc(loc, 'query') || isLoc(loc, 'attributes')) fields.attributes = msg
          else if (isCondition) fields.condition = msg
          else if (isLoc(loc, 'first')) fields.first = msg
          else if (isLoc(loc, 'last')) fields.last = msg
          else if (isLoc(loc, 'num_results')) fields.limit = msg
          else if (isLoc(loc, 'sort_by')) fields.sort_by = msg
        }
        const errs: any[] = (e as any).problem.errors as any[]
        const first = errs[0] || {}
        const msgText =
          String(first?.message || '')
            .toLowerCase()
            .includes('unexpected property') && first?.location
            ? `Unexpected property: ${first.location}`
            : 'API request failed: validation failed'
        banner = { message: msgText, problem: (e as any).problem, status: (e as any).status }
      } else {
        // Special-case mapping for non-problem errors
        const status = (e as any)?.status
        let combined = ''
        const prob: any = (e as any)?.problem
        if (prob) {
          if (typeof prob.detail === 'string') combined += ' ' + prob.detail
          if (Array.isArray(prob.errors)) {
            combined += ' ' + prob.errors.map((er: any) => String(er?.message || '')).join(' ')
          }
        }
        const body: any = (e as any)?.body
        if (typeof body === 'string') combined += ' ' + body
        else if (body && typeof body === 'object' && typeof body.message === 'string')
          combined += ' ' + body.message
        const lc = normalizeText(combined)
        if (
          lc.includes("couldn't prepare query: list of target hosts is empty") ||
          (status === 500 && lc.includes('list of target hosts is empty'))
        ) {
          fields.hosts = 'List of target hosts is empty'
          banner = { message: 'API request failed: validation failed', problem: prob, status }
        }
      }
      return { fields, banner }
    },
    []
  )

  // Build effective params merging uncommitted inputs and normalizing attribute query
  const computeFinalParams = useCallback(
    (over?: { conditionOverride?: string; hostsOverride?: string }): QueryParamsUI => {
      // include any uncommitted condition input
      const effectiveParamsBase =
        (over?.conditionOverride ?? conditionInput) !== (params.condition || '')
          ? { ...params, condition: (over?.conditionOverride ?? conditionInput) || undefined }
          : params
      // include uncommitted Hosts input (normalized)
      const mergedHosts =
        (over?.hostsOverride ?? hostsInput) !== (effectiveParamsBase.query_hosts || '')
          ? sanitizeHostList(over?.hostsOverride ?? hostsInput)
          : effectiveParamsBase.query_hosts
      const effectiveParams =
        mergedHosts !== effectiveParamsBase.query_hosts
          ? { ...effectiveParamsBase, query_hosts: mergedHosts }
          : effectiveParamsBase
      // normalize attributes query: when 'All', send explicit full list (backend requires min length)
      const normalizedQuery = buildAttributeQuery(
        parseAttributeQuery(effectiveParams.query).values,
        parseAttributeQuery(effectiveParams.query).all
      )
      const finalParams =
        normalizedQuery === effectiveParams.query
          ? effectiveParams
          : { ...effectiveParams, query: normalizedQuery }
      return finalParams
    },
    [params, conditionInput, hostsInput]
  )

  async function validateCurrent(over?: { conditionOverride?: string; hostsOverride?: string }) {
    try {
      const finalParams = computeFinalParams(over)
      // set backend dynamically for the validator as well
      setGlobalQueryBaseUrl(backendUrl)
      // abort any in-flight validation
      try {
        validateAbortRef.current?.abort()
      } catch {}
      const ctrl = new AbortController()
      validateAbortRef.current = ctrl
      await getGlobalQueryClient().validateQueryUI(
        { ...finalParams, hosts_resolver: hostsResolver || undefined },
        ctrl.signal
      )
      // success: clear field and banner errors
      setFieldErrors({})
      setError('')
      return true
    } catch (e: any) {
      const mapped = mapValidationError(e)
      if (Object.keys(mapped.fields).length > 0) setFieldErrors(mapped.fields)
      setError(mapped.banner)
      return false
    }
  }

  useEffect(() => {
    const search = serializeParams(params)
    const url = new URL(window.location.href)
    url.search = search
    window.history.replaceState({}, '', url.toString())
  }, [params])

  const run = useCallback(async () => {
    // if we just canceled, wait a brief moment to let the transport settle
    const sinceCancel = Date.now() - (lastCancelAtRef.current || 0)
    if (sinceCancel >= 0 && sinceCancel < 120) {
      await new Promise((r) => setTimeout(r, 120 - sinceCancel))
    }
    // cancel any previous stream
    if (streamCloserRef.current) {
      try {
        streamCloserRef.current.close()
      } catch {}
      streamCloserRef.current = null
    }
    // cancel any previous non-streaming request
    if (runAbortRef.current) {
      try {
        runAbortRef.current.abort()
      } catch {}
      runAbortRef.current = null
    }
    setLoading(true)
    setStreaming(!!useStreaming)
    // don't clear errors yet; wait for validation OK
    setStreamErrors([])
    setProgress({})
    setHostsStatuses({})
    setHostErrorCount(0)
    setHostOkCount(0)
    try {
      const finalParams = computeFinalParams()
      if (finalParams !== params) {
        // sync params state (will also update URL)
        setParams(finalParams)
      }
      // set backend dynamically
      setGlobalQueryBaseUrl(backendUrl)
      setRows([])
      setSummary(undefined)
      // Preflight validate; only proceed when valid
      const valid = await validateCurrent()
      if (!valid) {
        setLoading(false)
        setStreaming(false)
        return
      }
      // clear any leftover errors now that validation passed
      setError('')
      setFieldErrors({})
      if (useStreaming) {
        // start SSE stream; server will send partialResult events until finalResult
        const closer = getGlobalQueryClient().streamQueryUI(
          { ...finalParams, hosts_resolver: hostsResolver || undefined },
          {
            onPartial: (flows, sum) => {
              // server may emit partial updates with rows=null (no row data yet); only replace when we actually have rows
              if (Array.isArray(flows) && flows.length > 0) {
                setRows(flows)
              }
              if (sum) setSummary(sum)
            },
            onFinal: (flows, sum) => {
              // Always replace with the final, server-sorted result so sort_ascending takes effect
              setRows(flows)
              if (sum) setSummary(sum)
              setStreaming(false)
              setLoading(false)
              streamCloserRef.current = null
            },
            onError: (er: any) => {
              const msg = typeof er?.message === 'string' ? er.message : 'stream error'
              const host = typeof er?.host === 'string' ? er.host : undefined
              setStreamErrors((prev) => [...prev, { message: msg, host }])
            },
            onProgress: (p) => setProgress(p || {}),
            onMeta: (meta) => {
              if (meta?.hostsStatuses) setHostsStatuses(meta.hostsStatuses)
              if (typeof meta?.hostErrorCount === 'number') setHostErrorCount(meta.hostErrorCount)
              if (typeof meta?.hostOkCount === 'number') setHostOkCount(meta.hostOkCount)
            },
          }
        )
        streamCloserRef.current = closer
      } else {
        // normal non-streaming request to /_query
        try {
          const ctrl = new AbortController()
          runAbortRef.current = ctrl
          const data = await getGlobalQueryClient().runQueryUI(
            {
              ...finalParams,
              hosts_resolver: hostsResolver || undefined,
            },
            ctrl.signal
          )
          setRows(data.flows)
          setSummary(data.summary)
          if (data.hostsStatuses) {
            setHostsStatuses(data.hostsStatuses)
            let err = 0,
              ok = 0
            for (const k of Object.keys(data.hostsStatuses)) {
              const c = String((data.hostsStatuses as any)[k]?.code || '').toLowerCase()
              if (c === 'ok') ok++
              else err++
            }
            setHostErrorCount(err)
            setHostOkCount(ok)
          }
        } finally {
          runAbortRef.current = null
          setLoading(false)
          setStreaming(false)
        }
      }
    } catch (e: any) {
      // extract problem+json field errors to map inputs
      if (
        e &&
        typeof e === 'object' &&
        (e as any).problem &&
        Array.isArray((e as any).problem.errors)
      ) {
        const fe: Record<string, string> = {}
        const isLoc = (loc: string, key: string) =>
          loc === `body.${key}` ||
          loc.startsWith(`body.${key}.`) ||
          loc.startsWith(`body.${key}[`) ||
          loc === key
        for (const er of (e as any).problem.errors as any[]) {
          const locRaw = String(er.location || '')
          const loc = locRaw.toLowerCase()
          const rawMsg = String(er.message || 'validation error').trim()
          // keep condition messages formatting (multi-line caret pointers), but capitalize first letter
          const isCondition = isLoc(loc, 'condition')
          let msg = isCondition
            ? rawMsg.replace(
                /^(\s*)([a-z])/,
                (_m: string, ws: string, ch: string) => ws + ch.toUpperCase()
              )
            : rawMsg.charAt(0).toUpperCase() + rawMsg.slice(1)
          // append value context only for non-condition fields to avoid duplicating formatted output
          if (!isCondition && er.value !== undefined) msg += ` -- value: ${formatValue(er.value)}`
          // HACK: if backend returns this specific text, attribute to Hosts Query
          const normRaw = normalizeText(rawMsg)
          if (
            normRaw === 'list of target hosts is empty' ||
            normRaw === "couldn't prepare query: list of target hosts is empty" ||
            normRaw.includes('list of target hosts is empty')
          ) {
            fe.hosts = msg
            continue
          }
          const isResolverField =
            isLoc(loc, 'query_hosts_resolver_type') || isLoc(loc, 'hosts_resolver')
          const isHostsField =
            isLoc(loc, 'query_hosts') || isLoc(loc, 'hostname') || isLoc(loc, 'host_id')
          if (isLoc(loc, 'ifaces')) fe.ifaces = msg
          // do not attach resolver field errors to any single input; keep in banner details only
          else if (!isResolverField && isHostsField) fe.hosts = msg
          else if (isLoc(loc, 'query') || isLoc(loc, 'attributes')) fe.attributes = msg
          else if (isCondition) fe.condition = msg
          else if (isLoc(loc, 'first')) fe.first = msg
          else if (isLoc(loc, 'last')) fe.last = msg
          else if (isLoc(loc, 'num_results')) fe.limit = msg
          else if (isLoc(loc, 'sort_by')) fe.sort_by = msg
        }
        setFieldErrors(fe)
        // Special-case: unexpected property => friendly message with location
        const errs: any[] = (e as any).problem.errors as any[]
        const first = errs[0] || {}
        const msgText =
          String(first?.message || '')
            .toLowerCase()
            .includes('unexpected property') && first?.location
            ? `Unexpected property: ${first.location}`
            : 'API request failed: validation failed'
        setError({
          message: msgText,
          problem: (e as any).problem,
          status: (e as any).status,
        })
      }
    } finally {
      // if in streaming mode, final handler turns off; otherwise already cleared
    }
  }, [params, conditionInput, hostsInput, backendUrl, useStreaming, hostsResolver])

  // Allow user to cancel an in-flight query (both streaming and non-streaming)
  const cancelRun = useCallback(() => {
    lastCancelAtRef.current = Date.now()
    try {
      // stop SSE stream if active
      if (streamCloserRef.current) {
        streamCloserRef.current.close()
        streamCloserRef.current = null
      }
    } catch {}
    try {
      // abort fetch for non-streaming
      if (runAbortRef.current) {
        runAbortRef.current.abort()
        runAbortRef.current = null
      }
    } catch {}
    try {
      // abort any in-flight validation to avoid surfacing an AbortError banner
      if (validateAbortRef.current) {
        validateAbortRef.current.abort()
        validateAbortRef.current = null
      }
    } catch {}
    // clear transient UI state
    setError('')
    setFieldErrors({})
    setStreamErrors([])
    setProgress({})
    setHostsStatuses({})
    setHostErrorCount(0)
    setHostOkCount(0)
    setLoading(false)
    setStreaming(false)
  }, [])

  // Auto-run only for non-streaming; for SSE, run only when the user presses the Run button
  useEffect(() => {
    if (!useStreaming) {
      void run()
    }
    // We intentionally exclude `run` and `conditionInput` to avoid validating on every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [params, backendUrl, hostsResolver, useStreaming])

  // open temporal details for a specific row (shared by click and keyboard shortcut)
  const openTemporalForRow = useCallback(
    async (r: FlowRecord) => {
      const condParts: string[] = []
      if (r.sip) condParts.push(`sip=${r.sip}`)
      if (r.dip) condParts.push(`dip=${r.dip}`)
      if (r.dport !== null && r.dport !== undefined) condParts.push(`dport=${r.dport}`)
      if (r.proto !== null && r.proto !== undefined) condParts.push(`proto=${r.proto}`)
      const condition = condParts.join(' and ')
      const hostId = r.host_id || ''
      const iface = r.iface || ''
      const meta = {
        host: r.host || hostId,
        iface,
        sip: r.sip || '',
        dip: r.dip || '',
        dport: r.dport,
        proto: r.proto,
      }
      const attrsShown = attrState.all
        ? ['sip', 'dip', 'dport', 'proto']
        : attrState.values.map((v) => (v === 'protocol' ? 'proto' : v === 'port' ? 'dport' : v))
      closeAllDetails()
      setTemporalDetail({ meta, attrsShown, rows: [], loading: true })
      try {
        const detailParams: QueryParamsUI = {
          ...params,
          query: 'time',
          condition: condition || undefined,
          query_hosts: hostId || undefined,
          ifaces: iface || '',
          limit: 100000,
          sort_by: 'bytes',
          sort_ascending: false,
        }
        const data = await getGlobalQueryClient().runQueryUI({
          ...detailParams,
          hosts_resolver: 'string',
        })
        setTemporalDetail({
          meta,
          attrsShown,
          rows: data.flows,
          summary: data.summary,
          loading: false,
        })
      } catch (e: any) {
        setTemporalDetail({
          meta,
          attrsShown,
          rows: [],
          loading: false,
          error: e,
        })
      }
    },
    [params, attrState]
  )

  // Enter opens temporal details for the first row if in table view and no panel is open
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        const anyDetail = ipDetail || ifaceDetail || hostDetail || temporalDetail
        if (!anyDetail && activeTab === 'table' && rows.length > 0) {
          e.preventDefault()
          void openTemporalForRow(rows[0])
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeTab, rows, ipDetail, ifaceDetail, hostDetail, temporalDetail, openTemporalForRow])

  // simple saved views (localStorage)
  const [savedViews, setSavedViews] = useState<Array<{ name: string; params: QueryParamsUI }>>(
    () => {
      try {
        const raw = JSON.parse(localStorage.getItem('goquery_ui_views') || '[]')
        if (!Array.isArray(raw)) return []
        return raw.map((v: any) => ({
          name: String(v?.name ?? ''),
          params: sanitizeUIParams(v?.params),
        }))
      } catch {
        return []
      }
    }
  )
  function persistViews(next: Array<{ name: string; params: QueryParamsUI }>) {
    setSavedViews(next)
    try {
      localStorage.setItem('goquery_ui_views', JSON.stringify(next))
    } catch {}
  }
  const [saveViewName, setSaveViewName] = useState<string>('')
  function onSaveView() {
    const name = (saveViewName || '').trim()
    if (!name) return
    const next = [...savedViews.filter((v) => v.name !== name), { name, params }]
    persistViews(next)
    setSaveViewName('')
  }
  function onLoadView(name: string) {
    const found = savedViews.find((v) => v.name === name)
    if (!found) return
    setParams(sanitizeUIParams(found.params))
  }
  function exportCSV() {
    if (!rows.length) return
    const anyHost = rows.some((r) => !!r.host)
    const anyIface = rows.some((r) => !!r.iface)
    const shown = attrState.all
      ? ['sip', 'dip', 'dport', 'proto']
      : attrState.values.map((v) => (v === 'protocol' ? 'proto' : v === 'port' ? 'dport' : v))
    const headers = [
      ...(anyHost ? ['host'] : []),
      ...(anyIface ? ['iface'] : []),
      ...shown,
      'bytes_in',
      'bytes_out',
      'bytes_total',
      'packets_in',
      'packets_out',
      'packets_total',
    ]
    const escape = (v: any) => {
      if (v === null || v === undefined) return ''
      const s = String(v)
      if (s.includes(',') || s.includes('"') || s.includes('\n'))
        return '"' + s.replace(/"/g, '""') + '"'
      return s
    }
    const lines = [headers.join(',')]
    for (const r of rows) {
      const values: any[] = []
      if (anyHost) values.push(r.host || '')
      if (anyIface) values.push(r.iface || '')
      for (const a of shown) values.push((r as any)[a] ?? '')
      const bt = (r.bytes_in || 0) + (r.bytes_out || 0)
      const pt = (r.packets_in || 0) + (r.packets_out || 0)
      values.push(r.bytes_in || 0, r.bytes_out || 0, bt, r.packets_in || 0, r.packets_out || 0, pt)
      lines.push(values.map(escape).join(','))
    }
    const blob = new Blob([lines.join('\n')], {
      type: 'text/csv;charset=utf-8;',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'goquery-export.csv'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  }

  // query for IP details (proto,dport) when opened
  const openIpDetails = useCallback(
    async (ip: string) => {
      // close other panels
      setIfaceDetail(null)
      setHostDetail(null)
      setTemporalDetail(null)
      setIpDetail({ ip, rows: [], loading: true })
      try {
        // honor existing condition: concatenate with AND
        const baseCondRaw = (conditionInput || params.condition || '').trim()
        const ipCond = `host=${ip}`
        const combinedCond = baseCondRaw ? `(${baseCondRaw}) and (${ipCond})` : ipCond
        const detailParams: QueryParamsUI = {
          ...params,
          query: 'proto,dport',
          condition: combinedCond,
          limit: Math.max(1, params.limit || 1),
          sort_by: 'bytes',
          sort_ascending: false,
        }
        const data = await getGlobalQueryClient().runQueryUI({
          ...detailParams,
          hosts_resolver: 'string',
        })
        setIpDetail({
          ip,
          rows: data.flows,
          summary: data.summary,
          loading: false,
        })
      } catch (e: any) {
        setIpDetail({ ip, rows: [], loading: false, error: e })
      }
    },
    [params, conditionInput]
  )

  // query for Interface details: attributes iface,port,protocol; scope by host_id and selected iface
  const openIfaceDetails = useCallback(
    async (hostId: string, iface: string) => {
      // close other panels
      setIpDetail(null)
      setHostDetail(null)
      setTemporalDetail(null)
      if (!hostId || !iface) {
        setIfaceDetail({
          host: hostId || '(unknown)',
          iface: iface || '(iface)',
          rows: [],
          loading: false,
          error: 'Missing host or interface for details',
        })
        return
      }
      // resolve human-readable host name from current rows for display
      const displayHost = rows.find((r) => r.host_id === hostId)?.host || hostId
      setIfaceDetail({ host: displayHost, iface, rows: [], loading: true })
      try {
        const detailParams: QueryParamsUI = {
          ...params,
          // per requirements: attributes = iface,port,protocol; limit scope via host_id and selected interface inputs
          query: 'iface,port,protocol',
          query_hosts: hostId,
          ifaces: iface,
          condition: undefined,
          limit: Math.max(1, params.limit || 1),
          sort_by: 'bytes',
          sort_ascending: false,
        }
        const data = await getGlobalQueryClient().runQueryUI({
          ...detailParams,
          hosts_resolver: 'string',
        })
        setIfaceDetail({
          host: displayHost,
          iface,
          rows: data.flows,
          summary: data.summary,
          loading: false,
        })
      } catch (e: any) {
        setIfaceDetail({
          host: displayHost,
          iface,
          rows: [],
          loading: false,
          error: e,
        })
      }
    },
    [params, conditionInput, rows]
  )

  // host details: show interfaces grouped, query attributes: ifaces, scoped by host_id
  const openHostDetails = useCallback(
    async (hostId: string) => {
      // close other panels
      setIpDetail(null)
      setIfaceDetail(null)
      setTemporalDetail(null)
      if (!hostId) return
      const hostName = rows.find((r) => r.host_id === hostId)?.host || hostId
      setHostDetail({ hostId, hostName, rows: [], loading: true })
      try {
        const detailParams: QueryParamsUI = {
          ...params,
          query: 'iface',
          query_hosts: hostId,
          condition: undefined,
          limit: Math.max(1, params.limit || 1),
          sort_by: 'bytes',
          sort_ascending: false,
        }
        const data = await getGlobalQueryClient().runQueryUI({
          ...detailParams,
          hosts_resolver: 'string',
        })
        setHostDetail({
          hostId,
          hostName,
          rows: data.flows,
          summary: data.summary,
          loading: false,
        })
      } catch (e: any) {
        setHostDetail({ hostId, hostName, rows: [], loading: false, error: e })
      }
    },
    [params, rows]
  )

  function onTimePreset(minutes: number) {
    const lastDate = new Date()
    const firstDate = new Date(lastDate.getTime() - minutes * 60 * 1000)
    setParams((p) => ({
      ...p,
      first: firstDate.toISOString(),
      last: lastDate.toISOString(),
    }))
  }

  return (
    <div className="min-h-screen bg-surface text-gray-200">
      <header className="border-b border-white/10 bg-surface-100/60 backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-3">
          <div className="text-lg font-semibold tracking-tight text-white">
            Goquery / Network Usage
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-6 py-6">
        {/* Query Input Panel */}
        <div className="mb-6 rounded-lg border border-white/10 bg-surface-100/60 p-4 relative">
          <button
            type="button"
            onClick={() => setSettingsOpen(true)}
            className="absolute right-3 top-3 rounded-md bg-surface-200 px-2 py-1 text-[12px] font-medium ring-1 ring-white/10 hover:bg-surface-300 focus:outline-none focus:ring-primary-500"
          >
            Settings
          </button>
          <div className="space-y-4">
            {/* Row 1 */}
            <div className="grid grid-cols-12 gap-4">
              {/* Hosts Query */}
              <div className="col-span-12 md:col-span-4 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Hosts Query</label>
                <input
                  type="text"
                  placeholder="Free text query"
                  className={
                    `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.hosts
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-white/10 focus:ring-primary-500')
                  }
                  value={hostsInput}
                  onChange={(e) => setHostsInput(e.target.value)}
                  onBlur={commitHosts}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      commitHosts()
                    }
                  }}
                />
                {fieldErrors.hosts && (
                  <div className="mt-1 text-[11px] text-red-300">{fieldErrors.hosts}</div>
                )}
              </div>
              {/* Interfaces */}
              <div className="col-span-12 md:col-span-3 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Interfaces</label>
                <input
                  type="text"
                  placeholder="eth0,eth1"
                  className={
                    `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.ifaces
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-white/10 focus:ring-primary-500')
                  }
                  value={ifacesInput}
                  onChange={(e) => setIfacesInput(e.target.value)}
                  onBlur={commitInterfaces}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      commitInterfaces()
                    }
                  }}
                />
                {fieldErrors.ifaces && (
                  <div className="mt-1 text-[11px] text-red-300">{fieldErrors.ifaces}</div>
                )}
              </div>
              {/* Attributes */}
              <div className="col-span-12 md:col-span-3 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Attributes</label>
                <AttributesSelect
                  options={[
                    { label: 'Source IP', value: 'sip' },
                    { label: 'Destination IP', value: 'dip' },
                    { label: 'Port', value: 'dport' },
                    { label: 'IP Protocol', value: 'proto' },
                  ]}
                  value={attrState.values}
                  allSelected={attrState.all}
                  onChange={onAttributesChange}
                  hasError={!!fieldErrors.attributes}
                />
                {fieldErrors.attributes && (
                  <div className="mt-1 text-[11px] text-red-300">{fieldErrors.attributes}</div>
                )}
              </div>
              {/* spacer to keep grid alignment */}
              <div className="col-span-12 md:col-span-2" />
              {/* (Run button moved below panel) */}
            </div>
            {/* Row 2 - aligned under each primary field */}
            <div className="grid grid-cols-12 gap-4">
              {/* Time Range under Hosts */}
              <div className="col-span-12 md:col-span-4 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Time Range</label>
                <div className="grid grid-cols-6 gap-2">
                  {[5, 10, 30, 60, 360, 720, 1440, 2880, 10080, 43200, 129600, 259200].map((m) => {
                    let label: string
                    if (m < 60) label = `${m}m`
                    else if (m === 60) label = '1h'
                    else if (m < 1440) label = `${m / 60}h`
                    else if (m % 1440 === 0 && m / 1440 >= 2) label = `${m / 1440}d`
                    else label = `${m / 60}h`
                    return (
                      <button
                        key={m}
                        onClick={() => onTimePreset(m)}
                        className="w-full rounded-md bg-surface-200 px-2 py-1 text-center text-[13px] font-medium ring-1 ring-white/10 hover:bg-surface-300 focus:outline-none focus:ring-primary-500"
                      >
                        {label}
                      </button>
                    )
                  })}
                </div>
                {/* Manual From/To override presets */}
                <div className="mt-2 flex gap-2">
                  <div className="flex flex-col flex-1">
                    <label className="mb-0.5 text-[10px] tracking-wide text-gray-400">From</label>
                    <input
                      type="datetime-local"
                      className={
                        `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                        (fieldErrors.first
                          ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                          : 'ring-white/10 focus:ring-primary-500')
                      }
                      value={isoToLocalInput(params.first)}
                      onChange={(e) => {
                        const iso = localInputToIso(e.target.value)
                        if (iso) setParams((p) => ({ ...p, first: iso }))
                      }}
                    />
                    {fieldErrors.first && (
                      <div className="mt-1 text-[11px] text-red-300">{fieldErrors.first}</div>
                    )}
                  </div>
                  <div className="flex flex-col flex-1">
                    <label className="mb-0.5 text-[10px] tracking-wide text-gray-400">To</label>
                    <input
                      type="datetime-local"
                      className={
                        `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                        (fieldErrors.last
                          ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                          : 'ring-white/10 focus:ring-primary-500')
                      }
                      value={isoToLocalInput(params.last)}
                      onChange={(e) => {
                        const iso = localInputToIso(e.target.value)
                        if (iso) setParams((p) => ({ ...p, last: iso }))
                      }}
                    />
                    {fieldErrors.last && (
                      <div className="mt-1 text-[11px] text-red-300">{fieldErrors.last}</div>
                    )}
                  </div>
                </div>
              </div>
              {/* Sort By under Interfaces */}
              <div className="col-span-12 md:col-span-3 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Sort By</label>
                <select
                  className={
                    `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.sort_by
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-white/10 focus:ring-primary-500')
                  }
                  value={params.sort_by}
                  onChange={(e) =>
                    setParams((p) => ({
                      ...p,
                      sort_by: e.target.value as 'bytes' | 'packets',
                    }))
                  }
                >
                  <option value="bytes">Bytes</option>
                  <option value="packets">Packets</option>
                </select>
                {fieldErrors.sort_by && (
                  <div className="mt-1 text-[11px] text-red-300">{fieldErrors.sort_by}</div>
                )}
                <label className="mt-1 flex items-center gap-1 text-[11px] text-gray-400">
                  <input
                    type="checkbox"
                    checked={params.sort_ascending}
                    onChange={(e) =>
                      setParams((p) => ({
                        ...p,
                        sort_ascending: e.target.checked,
                      }))
                    }
                  />{' '}
                  Ascending
                </label>
              </div>
              {/* Limit under Attributes */}
              <div className="col-span-12 md:col-span-3 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Limit</label>
                <input
                  type="number"
                  min={1}
                  list="limit-presets"
                  className={
                    `w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.limit
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-white/10 focus:ring-primary-500')
                  }
                  value={params.limit}
                  onChange={(e) =>
                    setParams((p) => ({
                      ...p,
                      limit: Math.max(1, Number(e.target.value) || 1),
                    }))
                  }
                />
                {fieldErrors.limit && (
                  <div className="mt-1 text-[11px] text-red-300">{fieldErrors.limit}</div>
                )}
                <datalist id="limit-presets">
                  {[10, 25, 50, 100, 250, 500, 1000].map((n) => (
                    <option key={n} value={n} />
                  ))}
                </datalist>
              </div>
              {/* Spacer to maintain grid alignment for Run column */}
              <div className="col-span-12 md:col-span-2" />
            </div>
            {/* Row 3 - Condition full width */}
            <div className="grid grid-cols-12 gap-4">
              <div className="col-span-12 flex flex-col text-[11px]">
                <label className="mb-1 font-medium text-gray-400">Condition</label>
                <textarea
                  placeholder='Free text condition,  (e.g. "proto = TCP and (dport = 80 or dport = 443)")'
                  rows={2}
                  className={
                    `w-full resize-y rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.condition
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-white/10 focus:ring-primary-500')
                  }
                  value={conditionInput}
                  onChange={(e) => {
                    const v = e.target.value
                    setConditionInput(v)
                    if (v.endsWith(' ')) commitCondition(v)
                  }}
                  onBlur={() => commitCondition()}
                />
                {fieldErrors.condition && (
                  <pre className="mt-1 whitespace-pre-wrap text-[11px] leading-snug text-red-300 font-mono">
                    {fieldErrors.condition}
                  </pre>
                )}
              </div>
            </div>
          </div>
        </div>
        <ErrorBanner error={error} />
        <div className="mb-4 flex items-center justify-between gap-4">
          <div className="flex gap-2">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setActiveTab(t.id)}
                className={
                  'rounded-md px-3 py-1.5 text-sm font-medium ring-1 ring-white/10 transition-colors ' +
                  (activeTab === t.id
                    ? 'bg-primary-500 text-white'
                    : 'bg-surface-100 text-gray-300 hover:text-white hover:bg-surface-200')
                }
              >
                {t.label}
              </button>
            ))}
          </div>
          <div>
            <select
              className="mr-2 rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-white/10"
              onChange={(e) => e.target.value && onLoadView(e.target.value)}
              value=""
            >
              <option value="" disabled>
                Saved views…
              </option>
              {savedViews.map((v) => (
                <option key={v.name} value={v.name}>
                  {v.name}
                </option>
              ))}
            </select>
            <input
              type="text"
              placeholder="View name"
              className="mr-2 rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-white/10 focus:outline-none focus:ring-primary-500"
              value={saveViewName}
              onChange={(e) => setSaveViewName(e.target.value)}
            />
            <button
              onClick={onSaveView}
              disabled={!saveViewName.trim()}
              className="mr-2 rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-white/10 hover:bg-surface-200 disabled:opacity-50"
            >
              Save view
            </button>
            <button
              onClick={exportCSV}
              className="mr-2 rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-white/10 hover:bg-surface-200"
            >
              Export CSV
            </button>
            {loading ? (
              <button
                onClick={() => cancelRun()}
                className="inline-flex items-center gap-1 rounded-md px-3 py-1.5 text-sm font-medium bg-red-500/10 text-red-200 ring-1 ring-red-500/40 hover:bg-red-500/20 focus:outline-none focus:ring-2 focus:ring-red-500/40"
                title="Cancel the running query"
              >
                Cancel
              </button>
            ) : (
              <button onClick={() => run()} className="btn btn-primary" title="Run the query">
                Run
              </button>
            )}
          </div>
        </div>
        {summary && (
          <div className="mb-4 rounded-lg border border-white/10 bg-surface-100/60 p-4 text-[12px] relative">
            <div className="absolute right-2 top-2 flex items-center gap-2">
              <span className="text-[11px] text-gray-300">Copy results</span>
              {copiedToast && <span className="text-[11px] text-primary-300">Table copied</span>}
              <button
                type="button"
                className="rounded-md bg-surface-200 px-2 py-1 ring-1 ring-white/10 hover:bg-surface-300"
                title="Copy text table"
                onClick={async () => {
                  try {
                    const t: any = (summary as any)?.totals || {}
                    const text = buildTextTable(rows, {
                      attributes: attrState.all
                        ? undefined
                        : attrState.values.map((v) =>
                            v === 'protocol' ? 'proto' : v === 'port' ? 'dport' : v
                          ),
                      totalsBytes: (() => {
                        const br = t.br || 0,
                          bs = t.bs || 0
                        return br + bs
                      })(),
                      totalsPackets: (() => {
                        const pr = t.pr || 0,
                          ps = t.ps || 0
                        return pr + ps
                      })(),
                      meta: {
                        first: (summary as any)?.time_first,
                        last: (summary as any)?.time_last,
                        interfacesCount: Array.isArray(summary.interfaces)
                          ? summary.interfaces.length
                          : 0,
                        hostsTotal: (hostOkCount || 0) + (hostErrorCount || 0),
                        hostsOk: hostOkCount || 0,
                        hostsErrors: hostErrorCount || 0,
                        sortBy: params.sort_by,
                        hitsTotal: (summary as any)?.hits?.total,
                        durationNs: (summary as any)?.timings?.query_duration_ns,
                        br: t.br || 0,
                        bs: t.bs || 0,
                        pr: t.pr || 0,
                        ps: t.ps || 0,
                      },
                    })
                    if (navigator.clipboard?.writeText) {
                      await navigator.clipboard.writeText(text)
                    }
                    setCopiedToast(true)
                    window.setTimeout(() => setCopiedToast(false), 1500)
                  } catch {}
                }}
                aria-label="Copy text table"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                  className="h-4 w-4 text-gray-300"
                >
                  <path d="M16 1H4a2 2 0 0 0-2 2v12h2V3h12V1ZM20 5H8a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2Zm0 16H8V7h12v14Z" />
                </svg>
              </button>
            </div>
            <div className="mb-2 text-[11px] font-semibold tracking-wide text-gray-300 uppercase">
              Summary
            </div>
            <div
              className="grid gap-3"
              style={{
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
              }}
            >
              <SummaryStat
                label="Time Range"
                multiline
                value={
                  <div className="flex flex-col">
                    <span>{formatTimestamp(summary.time_first)}</span>
                    <span>{formatTimestamp(summary.time_last)}</span>
                    <span className="text-primary-300">
                      {humanRangeDuration(summary.time_first, summary.time_last)}
                    </span>
                  </div>
                }
              />
              {/* Hosts panel: two-line clamp list of queried hosts with counts */}
              <SummaryStat
                label="Hosts"
                multiline
                value={(() => {
                  const statusEntries = Object.entries(hostsStatuses || {})
                  const nameById: Record<string, string> = {}
                  for (const r of rows) {
                    if (r.host_id && r.host) nameById[r.host_id] = r.host
                  }
                  const hostNames: string[] =
                    statusEntries.length > 0
                      ? statusEntries.map(([id]) => nameById[id] || id)
                      : Array.from(new Set(rows.map((r) => r.host).filter((h): h is string => !!h)))
                  const hostsCount = hostOkCount + hostErrorCount
                  const errors = hostErrorCount
                  const summaryLine =
                    errors > 0
                      ? `${hostsCount} total, ${hostOkCount} ok, ${errors} error${errors === 1 ? '' : 's'}`
                      : `${hostsCount} total`
                  return (
                    <div className="flex flex-col">
                      <span
                        className="cursor-default"
                        style={{
                          display: '-webkit-box',
                          WebkitLineClamp: 2 as any,
                          WebkitBoxOrient: 'vertical',
                          overflow: 'hidden',
                        }}
                        title={hostNames.join(', ')}
                      >
                        {hostNames.length ? hostNames.join(', ') : '—'}
                      </span>
                      <button
                        type="button"
                        className="mt-0.5 text-left text-primary-300 hover:text-primary-200"
                        onClick={() => setIfaceDetailOpen(true)}
                        title="Show details"
                      >
                        {summaryLine}
                      </button>
                    </div>
                  )
                })()}
              />
              <SummaryStat
                label="Interfaces"
                multiline
                value={(() => {
                  const list = summary.interfaces || []
                  const total = list.length
                  return (
                    <div className="flex flex-col">
                      <span
                        className="cursor-pointer hover:underline decoration-primary-400/60"
                        style={{
                          display: '-webkit-box',
                          WebkitLineClamp: 2 as any,
                          WebkitBoxOrient: 'vertical',
                          overflow: 'hidden',
                        }}
                        title={list.join(', ')}
                        onClick={() => setIfaceDetailOpen(true)}
                      >
                        {list.length ? list.join(', ') : '—'}
                      </span>
                      <button
                        type="button"
                        className="text-left text-primary-300 hover:text-primary-200"
                        onClick={() => setIfaceDetailOpen(true)}
                        title={total > 0 ? `${total} total interfaces` : '—'}
                      >
                        {total > 0 ? `${total} total` : '—'}
                      </button>
                    </div>
                  )
                })()}
              />
              <SummaryStat
                label="Bytes (in/out)"
                multiline
                value={
                  <div className="flex flex-col">
                    <span>
                      {humanBytes((summary.totals as any).br ?? 0)} /{' '}
                      {humanBytes((summary.totals as any).bs ?? 0)}
                    </span>
                    <span className="text-primary-300">
                      {humanBytes(
                        ((summary.totals as any).br ?? 0) + ((summary.totals as any).bs ?? 0)
                      )}
                    </span>
                  </div>
                }
              />
              <SummaryStat
                label="Packets (in/out)"
                multiline
                value={
                  <div className="flex flex-col">
                    <span>
                      {humanPackets((summary.totals as any).pr ?? 0)} /{' '}
                      {humanPackets((summary.totals as any).ps ?? 0)}
                    </span>
                    <span className="text-primary-300">
                      {humanPackets(
                        ((summary.totals as any).pr ?? 0) + ((summary.totals as any).ps ?? 0)
                      )}
                    </span>
                  </div>
                }
              />
              {summary.timings.resolution !== undefined && (
                <SummaryStat
                  label="DNS Resolution"
                  value={formatDurationNs(summary.timings.resolution)}
                />
              )}
            </div>
          </div>
        )}
        {/* Host progress bar: below Summary, above Displayed count; full width, 4px height */}
        {(() => {
          const ok = hostOkCount || 0
          const err = hostErrorCount || 0
          const processed = ok + err
          if (processed <= 0) return null
          const okPct = Math.round((ok / processed) * 100)
          const errPct = 100 - okPct
          return (
            <div className="mb-2">
              <div className="h-1 w-full rounded-full bg-surface-300/70 flex">
                <div className="h-full bg-blue-500 rounded-full" style={{ width: okPct + '%' }} />
                <div
                  className="h-full bg-red-300 rounded-full cursor-pointer transform origin-center transition-transform duration-150 hover:scale-y-150"
                  style={{ width: errPct + '%' }}
                  title="Show host error details"
                  onClick={() => setIfaceDetailOpen(true)}
                />
              </div>
            </div>
          )
        })()}
        {(() => {
          const displayed = rows.length
          const total = summary?.hits?.total
          if (displayed === 0 && total === undefined) return null
          const stats: any = (summary as any)?.stats || {}
          const blocks: number | undefined =
            typeof stats.blocks_processed === 'number' ? stats.blocks_processed : undefined
          const decBytes: number | undefined =
            typeof stats.bytes_decompressed === 'number' ? stats.bytes_decompressed : undefined
          const durNs: number | undefined = (summary as any)?.timings?.query_duration_ns
          const hasLoaded = blocks !== undefined || decBytes !== undefined || durNs !== undefined
          return (
            <div className="mb-2 flex items-center justify-between text-[12px] text-gray-300">
              <DisplaySummary displayed={displayed} total={total} />
              {hasLoaded && (
                <div className="text-right text-[12px] text-gray-300">
                  Loaded{' '}
                  <span className="font-semibold text-white">{humanPackets(blocks ?? 0)}</span>{' '}
                  blocks /{' '}
                  <span className="font-semibold text-white">{humanBytes(decBytes ?? 0)}</span>{' '}
                  decompressed in{' '}
                  <span className="font-semibold text-white">{formatDurationNs(durNs ?? 0)}</span>
                </div>
              )}
            </div>
          )
        })()}
        <div className="relative rounded-lg ring-1 ring-white/10">
          {/* show blocking overlay only for non-streaming requests; allow partial results during streaming */}
          {loading && !streaming && (
            <div className="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-surface-100/70 backdrop-blur-[1px]">
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/20 border-t-white" />
            </div>
          )}
          {/* Settings detail overlay */}
          {settingsOpen && (
            <>
              <div
                className="absolute inset-0 z-20 rounded-lg bg-black/50"
                onClick={() => setSettingsOpen(false)}
              />
              <div className="absolute left-1/2 top-16 z-30 w-[min(520px,90%)] -translate-x-1/2 rounded-lg border border-white/10 bg-surface-100 p-4 shadow-xl">
                <div className="mb-2 flex items-center justify-between">
                  <div className="text-[13px] font-semibold text-gray-200">Settings</div>
                  <button
                    type="button"
                    onClick={() => setSettingsOpen(false)}
                    className="rounded-md bg-surface-200 px-2 py-1 text-[12px] ring-1 ring-white/10 hover:bg-surface-300"
                  >
                    Close
                  </button>
                </div>
                <div className="space-y-3 text-[12px]">
                  <div className="flex items-center justify-between">
                    <label className="flex items-center gap-2 text-gray-300">
                      <input
                        type="checkbox"
                        checked={useStreaming}
                        onChange={(e) => setUseStreaming(e.target.checked)}
                      />
                      Stream results
                    </label>
                    <button
                      type="button"
                      className="text-[11px] text-gray-400 hover:text-gray-200 underline decoration-dotted"
                      onClick={() => {
                        try {
                          localStorage.removeItem(LS_STREAMING_KEY)
                        } catch {}
                        setUseStreaming(!!env.SSE_ON_LOAD)
                      }}
                      title={`Reset to default (${env.SSE_ON_LOAD ? 'on' : 'off'})`}
                    >
                      Reset to default ({env.SSE_ON_LOAD ? 'on' : 'off'})
                    </button>
                  </div>
                  <div className="flex flex-col">
                    <label className="mb-1 text-[11px] tracking-wide text-gray-400">Backend</label>
                    <input
                      type="text"
                      placeholder={defaultBackend}
                      className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-white/10 focus:outline-none focus:ring-primary-500"
                      value={backendUrl}
                      onChange={(e) => setBackendUrl(e.target.value)}
                    />
                    <div className="mt-1 text-[11px] text-gray-500">Default: {defaultBackend}</div>
                  </div>
                  <div className="flex flex-col">
                    <label className="mb-1 text-[11px] tracking-wide text-gray-400">
                      Hosts Resolver
                    </label>
                    {env.HOST_RESOLVER_TYPES.length > 0 ? (
                      <select
                        className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-white/10 focus:outline-none focus:ring-primary-500"
                        value={hostsResolver}
                        onChange={(e) => setHostsResolver(e.target.value)}
                      >
                        {env.HOST_RESOLVER_TYPES.map((opt: string) => (
                          <option key={opt} value={opt}>
                            {opt}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        type="text"
                        className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-white/10"
                        value="—"
                        disabled
                      />
                    )}
                    {env.HOST_RESOLVER_TYPES.length === 0 && (
                      <div className="mt-1 text-[11px] text-gray-500">
                        No resolver types configured
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </>
          )}
          {/* Interfaces & Host Status modal */}
          {ifaceDetailOpen && (
            <>
              <div
                className="absolute inset-0 z-20 rounded-lg bg-black/50"
                onClick={() => setIfaceDetailOpen(false)}
              />
              <div className="absolute left-1/2 top-16 z-30 w-[min(1000px,95%)] -translate-x-1/2 rounded-lg border border-white/10 bg-surface-100 p-4 shadow-xl">
                <div className="mb-3 flex items-center justify-between">
                  <div className="text-[13px] font-semibold text-gray-200">
                    Interfaces &amp; Host Status
                  </div>
                  <button
                    type="button"
                    onClick={() => setIfaceDetailOpen(false)}
                    className="rounded-md bg-surface-200 px-2 py-1 text-[12px] ring-1 ring-white/10 hover:bg-surface-300"
                  >
                    Close
                  </button>
                </div>
                <div className="grid grid-cols-1 gap-4 md:grid-cols-3 text-[12px]">
                  <div className="min-h-[180px]">
                    <div className="mb-2 text-[11px] uppercase tracking-wide text-gray-400">
                      Interfaces
                    </div>
                    {(() => {
                      const ifaces: string[] = Array.isArray(summary?.interfaces)
                        ? (summary?.interfaces as string[])
                        : []
                      if (ifaces.length === 0) {
                        return (
                          <div className="rounded-md bg-surface-200/40 p-3 text-gray-400 ring-1 ring-white/5">
                            No interfaces found.
                          </div>
                        )
                      }
                      return (
                        <ul className="max-h-64 overflow-auto rounded-md bg-surface-200/40 p-2 ring-1 ring-white/5">
                          {ifaces
                            .slice()
                            .sort((a: string, b: string) => a.localeCompare(b))
                            .map((iface: string, i: number) => (
                              <li key={i} className="mb-1 last:mb-0 break-all text-gray-100">
                                {iface}
                              </li>
                            ))}
                        </ul>
                      )
                    })()}
                  </div>
                  <div className="min-h-[180px]">
                    <div className="mb-2 text-[11px] uppercase tracking-wide text-gray-400">
                      Host OK
                    </div>
                    {(() => {
                      const entries = Object.entries(hostsStatuses || {})
                        .filter(([, v]) => String(v?.code || '').toLowerCase() === 'ok')
                        .sort(([a], [b]) => a.localeCompare(b))
                      if (entries.length === 0) {
                        return (
                          <div className="rounded-md bg-surface-200/40 p-3 text-gray-400 ring-1 ring-white/5">
                            No OK hosts.
                          </div>
                        )
                      }
                      const nameById: Record<string, string> = {}
                      for (const r of rows) {
                        if (r.host_id && r.host) nameById[r.host_id] = r.host
                      }
                      return (
                        <ul className="max-h-64 overflow-auto rounded-md bg-surface-200/40 p-2 ring-1 ring-white/5">
                          {entries.map(([hostId], i) => (
                            <li key={hostId + i} className="mb-1 last:mb-0 text-gray-100">
                              {nameById[hostId] || hostId}
                              {nameById[hostId] && (
                                <span className="ml-2 font-mono text-[11px] text-gray-400">
                                  {hostId}
                                </span>
                              )}
                            </li>
                          ))}
                        </ul>
                      )
                    })()}
                  </div>
                  <div className="min-h-[180px]">
                    <div className="mb-2 text-[11px] uppercase tracking-wide text-gray-400">
                      Host Errors
                    </div>
                    {(() => {
                      const entries = Object.entries(hostsStatuses || {})
                        .filter(([, v]) => String(v?.code || '').toLowerCase() !== 'ok')
                        .sort(([a], [b]) => a.localeCompare(b))
                      if (entries.length === 0) {
                        return (
                          <div className="rounded-md bg-surface-200/40 p-3 text-gray-400 ring-1 ring-white/5">
                            No host errors.
                          </div>
                        )
                      }
                      const nameById: Record<string, string> = {}
                      for (const r of rows) {
                        if (r.host_id && r.host) nameById[r.host_id] = r.host
                      }
                      return (
                        <ul className="max-h-64 overflow-auto rounded-md bg-surface-200/40 p-2 ring-1 ring-white/5">
                          {entries.map(([hostId, st], i) => (
                            <li key={hostId + i} className="mb-2 last:mb-0">
                              <div className="font-medium text-gray-100">
                                {nameById[hostId] || hostId}
                                {nameById[hostId] && (
                                  <span className="ml-2 font-mono text-[11px] text-gray-400">
                                    {hostId}
                                  </span>
                                )}
                              </div>
                              <div className="mt-0.5 text-[11px] text-red-300">
                                <span className="uppercase text-[10px] tracking-wide text-red-400/80">
                                  {String(st?.code || 'error')}
                                </span>
                                {st?.message && (
                                  <span className="ml-2 text-gray-300">{st.message}</span>
                                )}
                              </div>
                            </li>
                          ))}
                        </ul>
                      )
                    })()}
                  </div>
                </div>
              </div>
            </>
          )}
          {(ipDetail || ifaceDetail || hostDetail || temporalDetail) && (
            <div
              className="absolute inset-0 z-10 rounded-lg bg-black/40 backdrop-blur-[1px]"
              onClick={closeAllDetails}
            />
          )}
          <div className="h-[70vh] overflow-auto scroll-thin mt-1">
            {activeTab === 'table' && (
              <TableView
                rows={rows}
                loading={loading && !streaming}
                streaming={streaming}
                attributes={
                  attrState.all
                    ? undefined
                    : attrState.values.map((v) =>
                        v === 'protocol' ? 'proto' : v === 'port' ? 'dport' : v
                      )
                }
                totalsBytes={(() => {
                  const t: any = (summary as any)?.totals || {}
                  const br = typeof t.br === 'number' ? t.br : 0
                  const bs = typeof t.bs === 'number' ? t.bs : 0
                  return br + bs
                })()}
                totalsPackets={(() => {
                  const t: any = (summary as any)?.totals || {}
                  const pr = typeof t.pr === 'number' ? t.pr : 0
                  const ps = typeof t.ps === 'number' ? t.ps : 0
                  return pr + ps
                })()}
                copyMeta={(() => {
                  const t: any = (summary as any)?.totals || {}
                  const hitsTotal = (summary as any)?.hits?.total
                  const durNs = (summary as any)?.timings?.query_duration_ns
                  const ifacesCount = Array.isArray((summary as any)?.interfaces)
                    ? ((summary as any)?.interfaces as any[]).length
                    : 0
                  const hostsTotal = (hostOkCount || 0) + (hostErrorCount || 0)
                  return {
                    first: params.first,
                    last: params.last,
                    interfacesCount: ifacesCount,
                    hostsTotal,
                    hostsOk: hostOkCount || 0,
                    hostsErrors: hostErrorCount || 0,
                    sortBy: params.sort_by,
                    hitsTotal: typeof hitsTotal === 'number' ? hitsTotal : undefined,
                    durationNs: typeof durNs === 'number' ? durNs : undefined,
                    br: typeof t.br === 'number' ? t.br : 0,
                    bs: typeof t.bs === 'number' ? t.bs : 0,
                    pr: typeof t.pr === 'number' ? t.pr : 0,
                    ps: typeof t.ps === 'number' ? t.ps : 0,
                  }
                })()}
                onRowClick={(r) => {
                  void openTemporalForRow(r)
                }}
              />
            )}
            {activeTab === 'graph' && (
              <div className="relative">
                <GraphView
                  rows={rows}
                  loading={loading && !streaming}
                  maxNodes={Math.max(100, params.limit || 0)}
                  onIpClick={(ip) => openIpDetails(ip)}
                  onIfaceClick={(host, iface) => openIfaceDetails(host, iface)}
                  onHostClick={(hostId) => openHostDetails(hostId)}
                />
                {ipDetail && (
                  <IpDetailsPanel
                    ip={ipDetail.ip}
                    rows={ipDetail.rows}
                    summary={ipDetail.summary}
                    loading={ipDetail.loading}
                    error={ipDetail.error}
                    onClose={() => setIpDetail(null)}
                  />
                )}
                {ifaceDetail && (
                  <IfaceDetailsPanel
                    host={ifaceDetail.host}
                    iface={ifaceDetail.iface}
                    rows={ifaceDetail.rows}
                    summary={ifaceDetail.summary}
                    loading={ifaceDetail.loading}
                    error={ifaceDetail.error}
                    onClose={() => setIfaceDetail(null)}
                  />
                )}
                {hostDetail && (
                  <HostDetailsPanel
                    host={hostDetail.hostName}
                    rows={hostDetail.rows}
                    summary={hostDetail.summary}
                    loading={hostDetail.loading}
                    error={hostDetail.error}
                    onClose={() => setHostDetail(null)}
                  />
                )}
              </div>
            )}
            {temporalDetail && (
              <TemporalDetailsPanel
                meta={temporalDetail.meta}
                attrsShown={temporalDetail.attrsShown}
                rows={temporalDetail.rows}
                summary={temporalDetail.summary}
                loading={temporalDetail.loading}
                error={temporalDetail.error}
                onClose={() => setTemporalDetail(null)}
              />
            )}
            {/* progress moved above; nothing sticky at the bottom anymore */}
          </div>
        </div>
      </main>
    </div>
  )
}
