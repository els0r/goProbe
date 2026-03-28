export function formatShortDuration(ms: number): string {
  const minutes = Math.round(ms / 60000)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.round(hours / 24)
  return `${days}d`
}

export function extractOffset(iso: string): { suffix: string; minutes: number } {
  const m = (iso || '').match(/(Z|[+-]\d{2}:?\d{2})$/)
  if (!m) return { suffix: '', minutes: 0 }
  const token = m[1]
  if (token === 'Z') return { suffix: 'Z', minutes: 0 }
  const sign = token[0] === '-' ? -1 : 1
  const hh = Number(token.slice(1, 3))
  const mm = Number(token.slice(-2))
  const suffix = `${sign === 1 ? '+' : '-'}${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`
  return { suffix, minutes: sign * (hh * 60 + mm) }
}

export function formatInOffset(
  ms: number,
  offsetMin: number,
  includeDate: boolean,
  includeSuffix: boolean,
  suffix: string
): string {
  const dt = new Date(ms + offsetMin * 60 * 1000)
  const Y = dt.getUTCFullYear()
  const M = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const D = String(dt.getUTCDate()).padStart(2, '0')
  const h = String(dt.getUTCHours()).padStart(2, '0')
  const min = String(dt.getUTCMinutes()).padStart(2, '0')
  const s = String(dt.getUTCSeconds()).padStart(2, '0')
  const datePart = `${Y}-${M}-${D} `
  const timePart = `${h}:${min}:${s}`
  return (includeDate ? datePart : '') + timePart + (includeSuffix ? suffix : '')
}

export function dateKeyInOffset(ms: number, offsetMin: number): string {
  const dt = new Date(ms + offsetMin * 60 * 1000)
  const Y = dt.getUTCFullYear()
  const M = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const D = String(dt.getUTCDate()).padStart(2, '0')
  return `${Y}-${M}-${D}`
}

export function buildIntervalLabel(
  endIso: string,
  durMs: number,
  prevEndIso?: string
): { label: string; isNewDate: boolean } {
  if (!endIso) return { label: '', isNewDate: false }
  const endMs = Date.parse(endIso)
  if (!Number.isFinite(endMs)) return { label: endIso, isNewDate: false }
  const { suffix, minutes } = extractOffset(endIso)
  const startMs = endMs - Math.max(1, durMs)
  const endDay = dateKeyInOffset(endMs, minutes)
  const prevDay =
    prevEndIso && Number.isFinite(Date.parse(prevEndIso))
      ? dateKeyInOffset(Date.parse(prevEndIso as string), minutes)
      : null
  const isNewDate = !prevDay || prevDay !== endDay
  const left = formatInOffset(startMs, minutes, isNewDate, isNewDate, suffix)
  const right = formatInOffset(endMs, minutes, false, false, '')
  return { label: `${left} – ${right}`, isNewDate }
}
