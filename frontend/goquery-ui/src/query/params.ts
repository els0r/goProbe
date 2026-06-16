// params.ts — the shape of a Query (CONTEXT.md): the parameter set one Run
// executes. Owned by the query/ concept module per ADR-0003; moved here from
// the transport-named api/domain.ts.

export interface QueryParamsUI {
  first: string
  last: string
  ifaces: string
  query: string
  // free text hosts query (separate from generic attribute list in `query`)
  query_hosts?: string
  // selected hosts resolver type from Settings; forwarded to backend as query_hosts_resolver_type
  hosts_resolver?: string
  condition?: string
  limit: number
  sort_by: 'bytes' | 'packets'
  sort_ascending: boolean
  in_only?: boolean
  out_only?: boolean
  sum?: boolean
}

// sanitize a comma-separated host list: trim items and drop empties
export function sanitizeHostList(raw?: string | null): string | undefined {
  if (!raw) return undefined
  const items = String(raw)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  return items.length ? items.join(',') : undefined
}

// the canonical default Query — the parameter set a fresh session starts from
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

// ensure any externally loaded params (e.g., from localStorage) are valid
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
