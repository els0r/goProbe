// Relative time range helpers. Ranges use the backend's native syntax:
// first="-3h" means "3 hours ago", an empty last means "now" (resolved
// server-side on every run, see pkg/query/time.go).
import { formatTimestamp } from './timeFormat'

export interface QuickRange {
  // value of `first` in backend syntax, e.g. "-3h"
  value: string
  label: string
}

export const QUICK_RANGES: QuickRange[] = [
  { value: '-5m', label: 'Last 5 minutes' },
  { value: '-10m', label: 'Last 10 minutes' },
  { value: '-30m', label: 'Last 30 minutes' },
  { value: '-1h', label: 'Last 1 hour' },
  { value: '-6h', label: 'Last 6 hours' },
  { value: '-12h', label: 'Last 12 hours' },
  { value: '-24h', label: 'Last 24 hours' },
  { value: '-2d', label: 'Last 2 days' },
  { value: '-7d', label: 'Last 7 days' },
  { value: '-30d', label: 'Last 30 days' },
  { value: '-90d', label: 'Last 90 days' },
  { value: '-180d', label: 'Last 180 days' },
]

export const DEFAULT_TIME_RANGE = { first: '-10m', last: '' }

export function isRelativeTime(s: string | undefined | null): boolean {
  return !!s && s.startsWith('-')
}

const UNIT_NAMES: Record<string, [string, string]> = {
  s: ['second', 'seconds'],
  m: ['minute', 'minutes'],
  h: ['hour', 'hours'],
  d: ['day', 'days'],
}

// Human-readable label for a range, e.g. "Last 3 hours" or
// "2026-05-04 14:15:38 → 2026-05-04 18:22:54".
export function describeTimeRange(first: string, last: string): string {
  if (isRelativeTime(first) && !last) {
    const preset = QUICK_RANGES.find((q) => q.value === first)
    if (preset) return preset.label
    const m = /^-(\d+)([smhd])$/.exec(first)
    if (m) {
      const n = Number(m[1])
      const [one, many] = UNIT_NAMES[m[2]]
      return `Last ${n} ${n === 1 ? one : many}`
    }
    // compound expressions like "-1d:12h" or "-1h30m"
    return `Last ${first.slice(1)}`
  }
  const from = first ? formatTimestamp(first) : '—'
  const to = last ? formatTimestamp(last) : 'now'
  return `${from} → ${to}`
}

// --- recently used absolute ranges (localStorage) ---

export interface AbsoluteRange {
  first: string
  last: string
}

const LS_RECENT_RANGES_KEY = 'goquery_ui_recent_ranges'
const MAX_RECENT_RANGES = 4

export function loadRecentRanges(): AbsoluteRange[] {
  try {
    const raw = JSON.parse(localStorage.getItem(LS_RECENT_RANGES_KEY) || '[]')
    if (!Array.isArray(raw)) return []
    return raw
      .filter((r: any) => typeof r?.first === 'string' && typeof r?.last === 'string')
      .slice(0, MAX_RECENT_RANGES)
  } catch {
    return []
  }
}

export function saveRecentRange(r: AbsoluteRange): AbsoluteRange[] {
  const next = [r, ...loadRecentRanges().filter((x) => x.first !== r.first || x.last !== r.last)]
    .slice(0, MAX_RECENT_RANGES)
  try {
    localStorage.setItem(LS_RECENT_RANGES_KEY, JSON.stringify(next))
  } catch { }
  return next
}

// Browser timezone info for the picker footer, e.g. ["Europe/Zurich", "UTC+02:00"]
export function browserTimeZone(): [string, string] {
  let zone = ''
  try {
    zone = Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch { }
  const offMin = -new Date().getTimezoneOffset()
  const sign = offMin >= 0 ? '+' : '-'
  const abs = Math.abs(offMin)
  const pad = (n: number) => String(n).padStart(2, '0')
  return [zone, `UTC${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`]
}

// Format a datetime-local input value ("2026-06-12T21:40") into the backend's
// offset-explicit absolute format ("2026-06-12 21:40 +0200") so parsing is
// unambiguous regardless of the server's timezone.
export function localInputToAbsolute(val: string): string | undefined {
  if (!val) return undefined
  const d = new Date(val)
  if (isNaN(d.getTime())) return undefined
  const pad = (n: number) => String(n).padStart(2, '0')
  const offMin = -d.getTimezoneOffset()
  const sign = offMin >= 0 ? '+' : '-'
  const abs = Math.abs(offMin)
  const offset = `${sign}${pad(Math.floor(abs / 60))}${pad(abs % 60)}`
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())} ${offset}`
}
