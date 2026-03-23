export function formatTimestamp(ts: string | undefined): string {
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

export function humanRangeDuration(startIso?: string | null, endIso?: string | null): string {
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
  if (d === 0 && h === 0 && m === 0) parts.push(s + 's')
  return parts.join('')
}

export function isoToLocalInput(iso?: string | null): string {
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

export function localInputToIso(val: string): string | undefined {
  if (!val) return undefined
  const d = new Date(val)
  if (isNaN(d.getTime())) return undefined
  return d.toISOString()
}
