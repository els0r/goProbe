import { formatValue, normalizeText } from './inputSanitize'

function isLoc(loc: string, key: string): boolean {
  return (
    loc === `body.${key}` ||
    loc.startsWith(`body.${key}.`) ||
    loc.startsWith(`body.${key}[`) ||
    loc === key
  )
}

export function mapValidationError(e: any): { fields: Record<string, string>; banner: any } {
  // Ignore user-initiated aborts
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
}
