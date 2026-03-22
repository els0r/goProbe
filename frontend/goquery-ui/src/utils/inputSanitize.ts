import { QueryParamsUI } from '../api/domain'

export function formatValue(v: unknown): string {
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
export function normalizeText(s: string | undefined | null): string {
  if (!s) return ''
  return String(s)
    .toLowerCase()
    .replace(/\u2019/g, "'")
    .trim()
}

// sanitize comma-separated host list: trim items and drop empties
export function sanitizeHostList(raw?: string | null): string | undefined {
  if (!raw) return undefined
  const items = String(raw)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  return items.length ? items.join(',') : undefined
}

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
export function sanitizeUIParams(p: any): QueryParamsUI {
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
