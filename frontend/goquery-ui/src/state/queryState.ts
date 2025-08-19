// URL query state serialization / parsing utilities
import { QueryParamsUI } from '../api/domain'

export function serializeParams(p: QueryParamsUI): string {
  const qp = new URLSearchParams()
  qp.set('first', p.first)
  qp.set('last', p.last)
  qp.set('ifaces', p.ifaces)
  qp.set('query', p.query)
  if (p.query_hosts) qp.set('query_hosts', p.query_hosts)
  qp.set('limit', String(p.limit))
  qp.set('sort_by', p.sort_by)
  qp.set('sort_ascending', String(p.sort_ascending))
  if (p.condition) qp.set('condition', p.condition)
  if (p.in_only) qp.set('in', 'true')
  if (p.out_only) qp.set('out', 'true')
  if (p.sum) qp.set('sum', 'true')
  return qp.toString()
}

export function parseParams(search: string): Partial<QueryParamsUI> {
  const sp = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search)
  const bool = (k: string) => sp.get(k) === 'true'
  const getNum = (k: string): number | undefined => {
    const v = sp.get(k)
    if (!v) return undefined
    const n = Number(v)
    return Number.isFinite(n) ? n : undefined
  }
  const out: Partial<QueryParamsUI> = {}
  const first = sp.get('first')
  if (first) out.first = first
  const last = sp.get('last')
  if (last) out.last = last
  const ifaces = sp.get('ifaces')
  if (ifaces) out.ifaces = ifaces
  const query = sp.get('query')
  if (query) out.query = query
  const query_hosts = sp.get('query_hosts')
  if (query_hosts) out.query_hosts = query_hosts
  const limit = getNum('limit')
  if (limit !== undefined) out.limit = limit
  const sort_by = sp.get('sort_by')
  if (sort_by === 'bytes' || sort_by === 'packets') out.sort_by = sort_by
  if (sp.get('sort_ascending')) out.sort_ascending = bool('sort_ascending')
  const condition = sp.get('condition')
  if (condition) out.condition = condition
  if (bool('in')) out.in_only = true
  if (bool('out')) out.out_only = true
  if (bool('sum')) out.sum = true
  return out
}
