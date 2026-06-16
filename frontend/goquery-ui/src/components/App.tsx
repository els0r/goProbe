import React, { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import {
  ThemePreference,
  readStoredPreference,
  applyTheme,
  storePreference,
  watchSystemTheme,
} from '../theme'
import { getGlobalQueryClient, setGlobalQueryBaseUrl } from '../api/client'
import { QueryRunner } from '../query/runner'
import { structuralKey, isDirty, runButtonState } from '../query/autoRun'
import { useDebouncedValue } from '../hooks/useDebouncedValue'
import { useQueryRunner } from '../query/useQueryRunner'
import { DetailRunner } from '../query/detailRunner'
import { useDetailRunner } from '../query/useDetailRunner'
import { FlowRecord, resultTotals } from '../flows'
import { QueryParamsUI, sanitizeHostList, sanitizeUIParams, parseParams, serializeParams } from '../query'
import { TableView } from '../views/TableView'
import { GraphView } from '../views/GraphView'
import { IpDetailsPanel } from '../views/IpDetailsPanel'
import { IfaceDetailsPanel } from '../views/IfaceDetailsPanel'
import { HostDetailsPanel } from '../views/HostDetailsPanel'
import {
  AttributesSelect,
  parseAttributeQuery,
  buildAttributeQuery,
  shownAttributes,
  AttributePreset,
} from './AttributesSelect'
import { Chevron } from './Chevron'
import { buildTextTable, buildCsv } from '../views/exportText'
import { env } from '../env'
import { formatDurationNs, humanBytes, humanPackets } from '../utils/format'
import { DisplaySummary } from './DisplaySummary'
import { ErrorBanner } from './ErrorBanner'
import { SummaryStat } from './SummaryStat'
import { SettingsModal } from './SettingsModal'
import { TimeRangePicker } from './TimeRangePicker'
import { formatTimestamp, humanRangeDuration } from '../utils/timeFormat'
import { DEFAULT_TIME_RANGE } from '../utils/timeRange'
import { mapValidationError } from '../utils/errorMapping'

const ATTR_PRESETS: AttributePreset[] = [
  { label: 'Top Talkers', values: ['sip', 'dip'], all: false },
  { label: 'Top Apps', values: ['dport', 'proto'], all: false },
  { label: 'Src IP', values: ['sip'], all: false },
  { label: 'Dst IP', values: ['dip'], all: false },
  { label: 'All', values: ['sip', 'dip', 'dport', 'proto'], all: true },
]

const CANCEL_REVEAL_MS = 450

function buildInitialParams(): QueryParamsUI {
  const parsed = parseParams(window.location.search)
  // default to "Top Talkers" (sip,dip) when no explicit query param provided
  const normalizedQuery = parsed.query
    ? buildAttributeQuery(
      parseAttributeQuery(parsed.query).values,
      parseAttributeQuery(parsed.query).all,
    )
    : 'sip,dip'
  return sanitizeUIParams({
    ...parsed,
    query_hosts: sanitizeHostList(parsed.query_hosts),
    query: normalizedQuery,
    // relative range, re-evaluated by the backend on every run ("" = now)
    first: parsed.first || DEFAULT_TIME_RANGE.first,
    last: parsed.last ?? DEFAULT_TIME_RANGE.last,
  })
}

const tabs = [
  { id: 'table', label: 'Table' },
  { id: 'graph', label: 'Graph' },
] as const

type TabId = (typeof tabs)[number]['id']

export default function App() {
  const [params, setParams] = useState<QueryParamsUI>(() => buildInitialParams())
  const [lastRunParams, setLastRunParams] = useState<QueryParamsUI | null>(null)
  // Cancel reveal: armed only after the threshold so fast auto-runs never flicker
  // Cancel; runNonce restarts the reveal timer on every Run, even superseding ones.
  const [cancelArmed, setCancelArmed] = useState(false)
  const [runNonce, setRunNonce] = useState(0)
  const [activeTab, setActiveTab] = useState<TabId>('table')
  // query orchestration is owned by the QueryRunner; App only observes its snapshot
  const [runner] = useState(() => new QueryRunner(getGlobalQueryClient))
  const snap = useQueryRunner(runner)
  const { rows, summary, hostsStatuses, hostOkCount, hostErrorCount } = snap
  // aggregate traffic totals, decoded once from the Summary's br/bs/pr/ps counters
  const totals = resultTotals(summary)
  const runInProgress = snap.phase === 'validating' || snap.phase === 'running'
  // blocking skeleton only while no (partial) rows can be shown
  const showSkeleton = runInProgress && !snap.partial
  // field-level errors and the banner are UI-shaped views over the runner's raw errors
  const { fieldErrors, error } = useMemo(() => {
    const source = snap.validationError ?? snap.runError
    if (!source) return { fieldErrors: {} as Record<string, string>, error: '' }
    const mapped = mapValidationError(source)
    return { fieldErrors: mapped.fields, error: mapped.banner }
  }, [snap.validationError, snap.runError])
  // Detail Runs and Drill-downs are owned by the DetailRunner; it observes the
  // QueryRunner so a new Run closes the panel by itself (ADR-0002)
  const [detailRunner] = useState(() => new DetailRunner(getGlobalQueryClient, runner))
  const detail = useDetailRunner(detailRunner)
  const ipDetail = detail.panel?.kind === 'ip' ? detail.panel : null
  const ifaceDetail = detail.panel?.kind === 'iface' ? detail.panel : null
  const hostDetail = detail.panel?.kind === 'host' ? detail.panel : null
  const temporalDetail = detail.panel?.kind === 'temporal' ? detail.panel : null
  // the selected table row IS the open temporal panel's target
  const selectedRow = temporalDetail?.row ?? null
  const [ifaceDetailOpen, setIfaceDetailOpen] = useState<boolean>(false)
  const [copiedToast, setCopiedToast] = useState<boolean>(false)
  // settings state
  const defaultBackend = env.GQ_API_BASE_URL
  const LS_BACKEND_KEY = 'goquery_ui_backend_url'
  const LS_STREAMING_KEY = 'goquery_ui_use_streaming'
  const LS_HOSTS_RESOLVER_KEY = 'goquery_ui_hosts_resolver'
  const LS_TOTALS_PCT_KEY = 'goquery_ui_show_totals_pct'
  const LS_INOUT_BARS_KEY = 'goquery_ui_visual_inout_bars'
  const LS_DIRECTION_VALUES_KEY = 'goquery_ui_show_direction_values'
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
    } catch { }
    // fallback to runtime env default
    return !!env.SSE_ON_LOAD
  })
  // Hosts Resolver selection; prefer a saved value if valid, else first available option
  const [hostsResolver, setHostsResolver] = useState<string>(() => {
    const opts = Array.isArray(env.HOST_RESOLVER_TYPES) ? env.HOST_RESOLVER_TYPES : []
    try {
      const saved = localStorage.getItem(LS_HOSTS_RESOLVER_KEY)
      if (typeof saved === 'string' && saved.length > 0 && opts.includes(saved)) return saved
    } catch { }
    return opts[0] || ''
  })
  const [settingsOpen, setSettingsOpen] = useState<boolean>(false)
  const [showTotalsPercentage, setShowTotalsPercentage] = useState<boolean>(() => {
    try {
      const saved = localStorage.getItem(LS_TOTALS_PCT_KEY)
      if (saved === '1' || saved === 'true') return true
      if (saved === '0' || saved === 'false') return false
    } catch { }
    return false
  })
  // visual diverging in/out bars; the design-overhaul default is ON, dense
  // textual columns remain one toggle away
  const [visualInOutBars, setVisualInOutBars] = useState<boolean>(() => {
    try {
      const saved = localStorage.getItem(LS_INOUT_BARS_KEY)
      if (saved === '1' || saved === 'true') return true
      if (saved === '0' || saved === 'false') return false
    } catch { }
    return true
  })
  // print the in/out figures that informed each bar's shape, flanking the bar;
  // an opt-in extension of the visual bars, off by default to keep them clean
  const [showDirectionValues, setShowDirectionValues] = useState<boolean>(() => {
    try {
      const saved = localStorage.getItem(LS_DIRECTION_VALUES_KEY)
      if (saved === '1' || saved === 'true') return true
      if (saved === '0' || saved === 'false') return false
    } catch { }
    return false
  })
  // theme preference (system | light | dark); resolved + applied via src/theme.ts
  const [themePreference, setThemePreference] = useState<ThemePreference>(() =>
    readStoredPreference()
  )
  // mirror of the current preference so the mount-only matchMedia listener can
  // re-resolve against the live value without re-subscribing
  const themePrefRef = useRef(themePreference)

  // persist backend selection and apply to client on change
  useEffect(() => {
    try {
      localStorage.setItem(LS_BACKEND_KEY, backendUrl)
    } catch { }
    setGlobalQueryBaseUrl(backendUrl)
  }, [backendUrl])

  // persist streaming preference
  useEffect(() => {
    try {
      localStorage.setItem(LS_STREAMING_KEY, useStreaming ? '1' : '0')
    } catch { }
  }, [useStreaming])

  // persist totals percentage preference
  useEffect(() => {
    try {
      localStorage.setItem(LS_TOTALS_PCT_KEY, showTotalsPercentage ? '1' : '0')
    } catch { }
  }, [showTotalsPercentage])

  // persist visual in/out bars preference
  useEffect(() => {
    try {
      localStorage.setItem(LS_INOUT_BARS_KEY, visualInOutBars ? '1' : '0')
    } catch { }
  }, [visualInOutBars])

  // persist direction-values preference
  useEffect(() => {
    try {
      localStorage.setItem(LS_DIRECTION_VALUES_KEY, showDirectionValues ? '1' : '0')
    } catch { }
  }, [showDirectionValues])

  // persist hosts resolver selection
  useEffect(() => {
    try {
      localStorage.setItem(LS_HOSTS_RESOLVER_KEY, hostsResolver)
    } catch { }
  }, [hostsResolver])

  // persist + apply theme preference, keeping the ref in sync for the
  // mount-only system listener below
  useEffect(() => {
    themePrefRef.current = themePreference
    storePreference(themePreference)
    applyTheme(themePreference)
  }, [themePreference])

  // track the OS colour scheme live; only re-apply while the preference is
  // 'system' so an explicit light/dark choice is never overridden by the OS
  useEffect(() => {
    return watchSystemTheme(() => {
      if (themePrefRef.current === 'system') applyTheme('system')
    })
  }, [])

  const onStreamingReset = useCallback(() => {
    try {
      localStorage.removeItem(LS_STREAMING_KEY)
    } catch { }
    setUseStreaming(!!env.SSE_ON_LOAD)
  }, [])

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
        if (detail.panel) {
          detailRunner.close()
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
  }, [detail.panel, settingsOpen, ifaceDetailOpen, detailRunner])
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
  function commitInterfaces() {
    setParams((p) => ({ ...p, ifaces: ifacesInput.trim() }))
  }
  function commitHosts() {
    setParams((p) => ({ ...p, query_hosts: sanitizeHostList(hostsInput) }))
  }
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

  // Params as they cross the runner seam: finalized inputs plus the resolver setting
  const paramsForBackend = useCallback(
    (over?: { conditionOverride?: string; hostsOverride?: string }): QueryParamsUI => ({
      ...computeFinalParams(over),
      hosts_resolver: hostsResolver || undefined,
    }),
    [computeFinalParams, hostsResolver]
  )

  function commitCondition(next?: string) {
    const raw = next ?? conditionInput
    setParams((p) => ({
      ...p,
      condition: raw.trim() ? raw : undefined,
    }))
    // Trigger validation immediately when committing from SPACE/blur
    void runner.validate(paramsForBackend({ conditionOverride: raw }))
  }

  useEffect(() => {
    const search = serializeParams(params)
    const url = new URL(window.location.href)
    url.search = search
    window.history.replaceState({}, '', url.toString())
  }, [params])

  const run = useCallback(() => {
    const finalParams = computeFinalParams()
    if (finalParams !== params) {
      // sync params state (will also update URL)
      setParams(finalParams)
    }
    // bump the nonce so each Run (even a superseding one) restarts the reveal timer
    setRunNonce((n) => n + 1)
    // snapshot at Run start: dirty is measured against what last ran
    setLastRunParams(finalParams)
    runner.run(paramsForBackend(), { stream: !!useStreaming })
  }, [params, computeFinalParams, paramsForBackend, useStreaming, runner])

  // Auto-run on structured input only (ADR-0005). Free-text edits flow into
  // params + the URL but never auto-run — structuralKey excludes them, so this
  // effect fires only on time range / attributes / sort / limit changes.
  // Debounced so a burst of structured edits collapses into one Run; the
  // runner's generation-based supersession is the backstop for any that still
  // overlap. Applies in both streaming and non-streaming modes.
  const debouncedStructKey = useDebouncedValue(structuralKey(params), 400)
  useEffect(() => {
    run()
    // run() reads the latest params/inputs via closure; listing it would re-fire
    // every render. Structured changes arrive via debouncedStructKey.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedStructKey, backendUrl, hostsResolver, useStreaming])

  // Reveal Cancel only after the threshold so fast auto-runs never flicker it.
  // Re-armed on each Run (runNonce) and cleared the moment a Run ends.
  useEffect(() => {
    if (!runInProgress) {
      setCancelArmed(false)
      return
    }
    setCancelArmed(false)
    const id = window.setTimeout(() => setCancelArmed(true), CANCEL_REVEAL_MS)
    return () => window.clearTimeout(id)
  }, [runInProgress, runNonce])

  // open temporal details for a specific row (shared by click and keyboard
  // shortcut); the runner toggles when the row is already open
  const openTemporalForRow = useCallback(
    (r: FlowRecord) => {
      detailRunner.open({ kind: 'temporal', row: r }, { params, rows })
    },
    [detailRunner, params, rows]
  )

  // Enter opens temporal details for the first row if in table view and no panel is open
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Enter') {
        if (!detail.panel && activeTab === 'table' && rows.length > 0) {
          e.preventDefault()
          openTemporalForRow(rows[0])
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeTab, rows, detail.panel, openTemporalForRow])

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
    } catch { }
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
    const csv = buildCsv(rows, { attributes: shownAttributes(attrState) })
    if (!csv) return
    const blob = new Blob([csv], {
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

  const onTimeRangeApply = useCallback((first: string, last: string) => {
    setParams((p) => ({ ...p, first, last }))
  }, [])

  // unapplied free-text edits vs the params of the last Run; computeFinalParams()
  // is what a Run would send, so both sides are normalized identically
  const dirty = isDirty(computeFinalParams(), lastRunParams)

  // The single Run control's morphing state: refresh | apply | busy | cancel.
  const btnState = runButtonState({
    phase: snap.phase,
    dirty,
    elapsedMs: cancelArmed ? CANCEL_REVEAL_MS : 0,
    threshold: CANCEL_REVEAL_MS,
  })

  return (
    <div className="min-h-screen bg-surface text-gray-200">
      {/* relative + z-40 lifts the header's stacking context above positioned
          page content (sticky table header z-10, dropdowns z-20) so the time
          picker popover paints on top */}
      <header className="relative z-40 border-b border-line bg-surface-100/60 backdrop-blur">
        <div className="mx-auto flex max-w-7xl items-center justify-between px-6 py-3">
          <div className="text-lg font-semibold tracking-tight text-gray-100">
            Goquery / Network Usage
          </div>
          <div className="flex items-center gap-3">
            <TimeRangePicker
              first={params.first}
              last={params.last}
              onApply={onTimeRangeApply}
              errors={{ first: fieldErrors.first, last: fieldErrors.last }}
            />
            {btnState === 'cancel' ? (
              <button onClick={() => runner.cancel()} className="btn btn-danger" title="Cancel the running query">
                Cancel
              </button>
            ) : btnState === 'busy' ? (
              <button disabled className="btn cursor-default opacity-90" title="Running…">
                <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current/40 border-t-current" />
                Run
              </button>
            ) : (
              <button
                onClick={() => run()}
                className={btnState === 'apply' ? 'btn btn-primary' : 'btn'}
                title={btnState === 'apply' ? 'Apply free-text edits and run' : 'Refresh — re-run the current query'}
              >
                Run
              </button>
            )}
            <button
              type="button"
              onClick={() => setSettingsOpen(true)}
              className="inline-flex items-center rounded-md bg-surface-200 px-2 py-1.5 ring-1 ring-line hover:bg-surface-300 focus:outline-none focus:ring-primary-500"
              title="Settings"
            >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 text-gray-300" viewBox="0 0 20 20" fill="currentColor">
              <path fillRule="evenodd" d="M7.84 1.804A1 1 0 0 1 8.82 1h2.36a1 1 0 0 1 .98.804l.331 1.652a6.993 6.993 0 0 1 1.929 1.115l1.598-.54a1 1 0 0 1 1.186.447l1.18 2.044a1 1 0 0 1-.205 1.251l-1.267 1.113a7.047 7.047 0 0 1 0 2.228l1.267 1.113a1 1 0 0 1 .206 1.25l-1.18 2.045a1 1 0 0 1-1.187.447l-1.598-.54a6.993 6.993 0 0 1-1.929 1.115l-.33 1.652a1 1 0 0 1-.98.804H8.82a1 1 0 0 1-.98-.804l-.331-1.652a6.993 6.993 0 0 1-1.929-1.115l-1.598.54a1 1 0 0 1-1.186-.447l-1.18-2.044a1 1 0 0 1 .205-1.251l1.267-1.114a7.05 7.05 0 0 1 0-2.227L1.821 7.773a1 1 0 0 1-.206-1.25l1.18-2.045a1 1 0 0 1 1.187-.447l1.598.54A6.992 6.992 0 0 1 7.51 3.456l.33-1.652ZM10 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z" clipRule="evenodd" />
            </svg>
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-6 py-6">
        {/* Query Input Panel */}
        <div className="mb-6 rounded-lg border border-line bg-surface-100/60 p-4">
          <div className="space-y-3 text-data-sm">
            {/* Row 1: Hosts Query | Interfaces | Attributes (+ presets) */}
            <div className="grid grid-cols-3 gap-3">
              <div className="flex flex-col min-w-0">
                <label className="mb-1 font-medium text-gray-400">Hosts Query</label>
                <input
                  type="text"
                  placeholder="Free text query"
                  className={
                    `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.hosts
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-line focus:ring-primary-500')
                  }
                  value={hostsInput}
                  onChange={(e) => setHostsInput(e.target.value)}
                  onBlur={commitHosts}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      run()
                    }
                  }}
                />
                {fieldErrors.hosts && (
                  <div className="mt-1 text-red-300">{fieldErrors.hosts}</div>
                )}
              </div>
              <div className="flex flex-col min-w-0">
                <label className="mb-1 font-medium text-gray-400">Interfaces</label>
                <input
                  type="text"
                  placeholder="eth0,eth1"
                  className={
                    `rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.ifaces
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-line focus:ring-primary-500')
                  }
                  value={ifacesInput}
                  onChange={(e) => setIfacesInput(e.target.value)}
                  onBlur={commitInterfaces}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      run()
                    }
                  }}
                />
                {fieldErrors.ifaces && (
                  <div className="mt-1 text-red-300">{fieldErrors.ifaces}</div>
                )}
              </div>
              <div className="flex flex-col min-w-0">
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
                  presets={ATTR_PRESETS}
                />
                {fieldErrors.attributes && (
                  <div className="mt-1 text-red-300">{fieldErrors.attributes}</div>
                )}
              </div>
            </div>

            {/* Row 2: Condition | Sort By + Ascending + Limit */}
            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2 flex flex-col min-w-0">
                <label className="mb-1 font-medium text-gray-400">Condition</label>
                <textarea
                  placeholder='Free text condition, e.g. "proto = TCP and (dport = 80 or dport = 443)"'
                  rows={1}
                  className={
                    `w-full resize-y rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                    (fieldErrors.condition
                      ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                      : 'ring-line focus:ring-primary-500')
                  }
                  value={conditionInput}
                  onChange={(e) => {
                    const v = e.target.value
                    setConditionInput(v)
                    if (v.endsWith(' ')) commitCondition(v)
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault()
                      run()
                    }
                  }}
                  onBlur={() => commitCondition()}
                />
                {fieldErrors.condition && (
                  <pre className="mt-1 whitespace-pre-wrap leading-snug text-red-300 font-mono">
                    {fieldErrors.condition}
                  </pre>
                )}
              </div>
              <div className="flex items-start gap-3 min-w-0">
                <div className="flex flex-col flex-1 min-w-0">
                  <label className="mb-1 font-medium text-gray-400">Sort By</label>
                  <div className="flex items-center gap-2">
                    <div className="relative flex-1">
                      <select
                        className={
                          `w-full appearance-none rounded-md bg-surface-200 px-2 py-1 pr-8 text-[13px] ring-1 focus:outline-none ` +
                          (fieldErrors.sort_by
                            ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                            : 'ring-line focus:ring-primary-500')
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
                      <Chevron className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2" />
                    </div>
                    <label className="flex items-center gap-1 text-gray-400 whitespace-nowrap">
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
                  {fieldErrors.sort_by && (
                    <div className="mt-1 text-red-300">{fieldErrors.sort_by}</div>
                  )}
                </div>
                <div className="flex flex-col w-[100px] shrink-0">
                  <label className="mb-1 font-medium text-gray-400">Limit</label>
                  <input
                    type="number"
                    min={1}
                    list="limit-presets"
                    className={
                      `w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 focus:outline-none ` +
                      (fieldErrors.limit
                        ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
                        : 'ring-line focus:ring-primary-500')
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
                    <div className="mt-1 text-red-300">{fieldErrors.limit}</div>
                  )}
                  <datalist id="limit-presets">
                    {[10, 25, 50, 100, 250, 500, 1000].map((n) => (
                      <option key={n} value={n} />
                    ))}
                  </datalist>
                </div>
              </div>
            </div>
          </div>
        </div>
        <ErrorBanner error={error} />
        <div className="mb-6 flex items-center justify-between gap-3">
          <div className="flex gap-3">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setActiveTab(t.id)}
                className={
                  'rounded-md px-3 py-1.5 text-sm font-medium ring-1 ring-line transition-colors ' +
                  (activeTab === t.id
                    ? 'bg-primary-500 text-on-accent'
                    : 'bg-surface-100 text-gray-300 hover:text-gray-100 hover:bg-surface-200')
                }
              >
                {t.label}
              </button>
            ))}
          </div>
          <div className="flex items-stretch gap-3">
            <div className="relative">
              <select
                className="h-full appearance-none rounded-md bg-surface-100 px-2 py-1 pr-8 text-[13px] ring-1 ring-line"
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
              <Chevron className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2" />
            </div>
            <input
              type="text"
              placeholder="View name"
              className="rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-line focus:outline-none focus:ring-primary-500"
              value={saveViewName}
              onChange={(e) => setSaveViewName(e.target.value)}
            />
            <button
              onClick={onSaveView}
              disabled={!saveViewName.trim()}
              className="rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-line hover:bg-surface-200 disabled:opacity-50"
            >
              Save view
            </button>
            <button
              onClick={exportCSV}
              className="rounded-md bg-surface-100 px-2 py-1 text-[13px] ring-1 ring-line hover:bg-surface-200"
            >
              Export CSV
            </button>
          </div>
        </div>
        {summary && (
          <div className="mb-6 rounded-lg border border-line bg-surface-100/60 p-4 text-data relative">
            <div className="absolute right-4 top-4 flex items-center gap-2">
              <span className="text-data-sm text-gray-300">Copy results</span>
              {copiedToast && <span className="text-data-sm text-accent">Table copied</span>}
              <button
                type="button"
                className="-m-1 rounded-md p-1 text-gray-400 hover:text-gray-200"
                title="Copy text table"
                onClick={async () => {
                  try {
                    const text = buildTextTable(rows, {
                      attributes: shownAttributes(attrState),
                      totalsBytes: totals.bytes_total,
                      totalsPackets: totals.packets_total,
                      meta: {
                        first: summary.time_first,
                        last: summary.time_last,
                        interfacesCount: Array.isArray(summary.interfaces)
                          ? summary.interfaces.length
                          : 0,
                        hostsTotal: (hostOkCount || 0) + (hostErrorCount || 0),
                        hostsOk: hostOkCount || 0,
                        hostsErrors: hostErrorCount || 0,
                        sortBy: params.sort_by,
                        hitsTotal: summary.hits?.total,
                        durationNs: summary.timings?.query_duration_ns,
                        br: totals.bytes_in,
                        bs: totals.bytes_out,
                        pr: totals.packets_in,
                        ps: totals.packets_out,
                      },
                    })
                    if (navigator.clipboard?.writeText) {
                      await navigator.clipboard.writeText(text)
                    }
                    setCopiedToast(true)
                    window.setTimeout(() => setCopiedToast(false), 1500)
                  } catch { }
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
            <div className="mb-2 text-data-sm font-semibold tracking-wide text-gray-300 uppercase">
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
                    <span className="text-accent">
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
                        className="mt-0.5 text-left text-accent hover:text-accent"
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
                        className="text-left text-accent hover:text-accent"
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
                      {humanBytes(totals.bytes_in)} /{' '}
                      {humanBytes(totals.bytes_out)}
                    </span>
                    <span className="text-accent">
                      {humanBytes(totals.bytes_total)}
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
                      {humanPackets(totals.packets_in)} /{' '}
                      {humanPackets(totals.packets_out)}
                    </span>
                    <span className="text-accent">
                      {humanPackets(totals.packets_total)}
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
                <div className="h-full bg-primary-500 rounded-full" style={{ width: okPct + '%' }} />
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
          const stats = summary?.stats
          const blocks = stats?.blocks_processed
          const decBytes = stats?.bytes_decompressed
          const durNs = summary?.timings?.query_duration_ns
          const hasLoaded = blocks !== undefined || decBytes !== undefined || durNs !== undefined
          return (
            <div className="mb-2 flex items-center justify-between text-data text-gray-300">
              <DisplaySummary displayed={displayed} total={total} />
              {hasLoaded && (
                <div className="text-right text-data text-gray-300">
                  Loaded{' '}
                  <span className="font-semibold text-gray-100">{humanPackets(blocks ?? 0)}</span>{' '}
                  blocks /{' '}
                  <span className="font-semibold text-gray-100">{humanBytes(decBytes ?? 0)}</span>{' '}
                  decompressed in{' '}
                  <span className="font-semibold text-gray-100">{formatDurationNs(durNs ?? 0)}</span>
                </div>
              )}
            </div>
          )
        })()}
        <div className="relative rounded-lg ring-1 ring-line">
          {/* blocking overlay only while no (partial) rows can be shown */}
          {showSkeleton && (
            <div className="absolute inset-0 z-10 flex items-center justify-center rounded-lg bg-surface-100/70 backdrop-blur-[1px]">
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-line-strong border-t-gray-100" />
            </div>
          )}
          {/* Settings overlay */}
          {settingsOpen && (
            <SettingsModal
              backendUrl={backendUrl}
              onBackendUrlChange={setBackendUrl}
              useStreaming={useStreaming}
              onStreamingChange={setUseStreaming}
              hostsResolver={hostsResolver}
              onHostsResolverChange={setHostsResolver}
              onStreamingReset={onStreamingReset}
              defaultBackend={defaultBackend}
              showTotalsPercentage={showTotalsPercentage}
              onTotalsPercentageChange={setShowTotalsPercentage}
              visualInOutBars={visualInOutBars}
              onVisualInOutBarsChange={setVisualInOutBars}
              showDirectionValues={showDirectionValues}
              onShowDirectionValuesChange={setShowDirectionValues}
              themePreference={themePreference}
              onThemePreferenceChange={setThemePreference}
              onClose={() => setSettingsOpen(false)}
            />
          )}
          {/* Interfaces & Host Status modal */}
          {ifaceDetailOpen && (
            <>
              <div
                className="absolute inset-0 z-20 rounded-lg bg-scrim"
                onClick={() => setIfaceDetailOpen(false)}
              />
              <div className="absolute left-1/2 top-16 z-30 w-[min(1000px,95%)] -translate-x-1/2 rounded-lg border border-line bg-surface-100 p-4 shadow-xl">
                <div className="mb-3 flex items-center justify-between">
                  <div className="text-[13px] font-semibold text-gray-200">
                    Interfaces &amp; Host Status
                  </div>
                  <button
                    type="button"
                    onClick={() => setIfaceDetailOpen(false)}
                    className="rounded-md bg-surface-200 px-2 py-1 text-data ring-1 ring-line hover:bg-surface-300"
                  >
                    Close
                  </button>
                </div>
                <div className="grid grid-cols-1 gap-4 md:grid-cols-3 text-data">
                  <div className="min-h-[180px]">
                    <div className="mb-2 text-data-sm uppercase tracking-wide text-gray-400">
                      Interfaces
                    </div>
                    {(() => {
                      const ifaces: string[] = Array.isArray(summary?.interfaces)
                        ? (summary?.interfaces as string[])
                        : []
                      if (ifaces.length === 0) {
                        return (
                          <div className="rounded-md bg-surface-200/40 p-3 text-gray-400 ring-1 ring-line-soft">
                            No interfaces found.
                          </div>
                        )
                      }
                      return (
                        <ul className="max-h-64 overflow-auto rounded-md bg-surface-200/40 p-2 ring-1 ring-line-soft">
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
                    <div className="mb-2 text-data-sm uppercase tracking-wide text-gray-400">
                      Host OK
                    </div>
                    {(() => {
                      const entries = Object.entries(hostsStatuses || {})
                        .filter(([, v]) => String(v?.code || '').toLowerCase() === 'ok')
                        .sort(([a], [b]) => a.localeCompare(b))
                      if (entries.length === 0) {
                        return (
                          <div className="rounded-md bg-surface-200/40 p-3 text-gray-400 ring-1 ring-line-soft">
                            No OK hosts.
                          </div>
                        )
                      }
                      const nameById: Record<string, string> = {}
                      for (const r of rows) {
                        if (r.host_id && r.host) nameById[r.host_id] = r.host
                      }
                      return (
                        <ul className="max-h-64 overflow-auto rounded-md bg-surface-200/40 p-2 ring-1 ring-line-soft">
                          {entries.map(([hostId], i) => (
                            <li key={hostId + i} className="mb-1 last:mb-0 text-gray-100">
                              {nameById[hostId] || hostId}
                              {nameById[hostId] && (
                                <span className="ml-2 text-data-sm text-gray-400">
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
                    <div className="mb-2 text-data-sm uppercase tracking-wide text-gray-400">
                      Host Errors
                    </div>
                    {(() => {
                      const entries = Object.entries(hostsStatuses || {})
                        .filter(([, v]) => String(v?.code || '').toLowerCase() !== 'ok')
                        .sort(([a], [b]) => a.localeCompare(b))
                      if (entries.length === 0) {
                        return (
                          <div className="rounded-md bg-surface-200/40 p-3 text-gray-400 ring-1 ring-line-soft">
                            No host errors.
                          </div>
                        )
                      }
                      const nameById: Record<string, string> = {}
                      for (const r of rows) {
                        if (r.host_id && r.host) nameById[r.host_id] = r.host
                      }
                      return (
                        <ul className="max-h-64 overflow-auto rounded-md bg-surface-200/40 p-2 ring-1 ring-line-soft">
                          {entries.map(([hostId, st], i) => (
                            <li key={hostId + i} className="mb-2 last:mb-0">
                              <div className="font-medium text-gray-100">
                                {nameById[hostId] || hostId}
                                {nameById[hostId] && (
                                  <span className="ml-2 text-data-sm text-gray-400">
                                    {hostId}
                                  </span>
                                )}
                              </div>
                              <div className="mt-0.5 text-data-sm text-red-300">
                                <span className="uppercase text-data-xs tracking-wide text-red-400/80">
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
          {(ipDetail || ifaceDetail || hostDetail) && (
            <div
              className="absolute inset-0 z-10 rounded-lg bg-scrim backdrop-blur-[1px]"
              onClick={() => detailRunner.close()}
            />
          )}
          <div className="h-[70vh] overflow-auto scroll-thin mt-1">
            {activeTab === 'table' && (
              <TableView
                rows={rows}
                loading={showSkeleton}
                streaming={snap.partial}
                attributes={shownAttributes(attrState)}
                totalsBytes={totals.bytes_total}
                totalsPackets={totals.packets_total}
                copyMeta={(() => {
                  const ifacesCount = Array.isArray(summary?.interfaces)
                    ? summary.interfaces.length
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
                    hitsTotal: summary?.hits?.total,
                    durationNs: summary?.timings?.query_duration_ns,
                    condition: params.condition,
                    br: totals.bytes_in,
                    bs: totals.bytes_out,
                    pr: totals.packets_in,
                    ps: totals.packets_out,
                  }
                })()}
                selectedRow={selectedRow}
                onRowClick={openTemporalForRow}
                showTotalsPercentage={showTotalsPercentage}
                visualInOutBars={visualInOutBars}
                showDirectionValues={showDirectionValues}
                temporalDetail={temporalDetail}
                drill={detail.drill}
                onDrill={(bucket, attrs) => detailRunner.openDrill(bucket, attrs)}
                onCloseDrill={() => detailRunner.closeDrill()}
              />
            )}
            {activeTab === 'graph' && (
              <div className="relative">
                <GraphView
                  rows={rows}
                  loading={showSkeleton}
                  maxNodes={Math.max(100, params.limit || 0)}
                  onIpClick={(ip) => detailRunner.open({ kind: 'ip', ip }, { params, rows })}
                  onIfaceClick={(hostId, iface) =>
                    detailRunner.open({ kind: 'iface', hostId, iface }, { params, rows })
                  }
                  onHostClick={(hostId) =>
                    detailRunner.open({ kind: 'host', hostId }, { params, rows })
                  }
                />
                {ipDetail && (
                  <IpDetailsPanel
                    ip={ipDetail.ip}
                    rows={ipDetail.rows}
                    summary={ipDetail.summary}
                    loading={ipDetail.phase === 'loading'}
                    error={ipDetail.error ?? undefined}
                    onClose={() => detailRunner.close()}
                  />
                )}
                {ifaceDetail && (
                  <IfaceDetailsPanel
                    host={ifaceDetail.hostName}
                    iface={ifaceDetail.iface}
                    rows={ifaceDetail.rows}
                    summary={ifaceDetail.summary}
                    loading={ifaceDetail.phase === 'loading'}
                    error={ifaceDetail.error ?? undefined}
                    onClose={() => detailRunner.close()}
                  />
                )}
                {hostDetail && (
                  <HostDetailsPanel
                    host={hostDetail.hostName}
                    rows={hostDetail.rows}
                    summary={hostDetail.summary}
                    loading={hostDetail.phase === 'loading'}
                    error={hostDetail.error ?? undefined}
                    onClose={() => detailRunner.close()}
                  />
                )}
              </div>
            )}
            {/* progress moved above; nothing sticky at the bottom anymore */}
          </div>
        </div>
      </main>
    </div>
  )
}
