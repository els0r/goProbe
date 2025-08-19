export function formatBytesIEC(bytes: number, fractionDigits = 1): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const v = bytes / Math.pow(k, i)
  return `${v.toFixed(fractionDigits)} ${units[i]}`
}

export function formatNumber(n: number): string {
  return n.toLocaleString(undefined)
}

// UI-standard human-friendly formatters
export function humanBytes(v: number | undefined | null): string {
  const n = typeof v === 'number' ? v : 0
  if (n < 1024) return n + ' B'
  const units = ['kB', 'MB', 'GB', 'TB', 'PB']
  let x = n / 1024
  let i = 0
  while (x >= 1024 && i < units.length - 1) { x /= 1024; i++ }
  return x.toFixed(x >= 100 ? 0 : x >= 10 ? 1 : 2) + ' ' + units[i]
}

export function humanPackets(v: number | undefined | null): string {
  const n = typeof v === 'number' ? v : 0
  if (n < 1000) return String(n)
  const units = ['K', 'M', 'B', 'T']
  let x = n
  let i = -1
  while (x >= 1000 && i < units.length - 1) { x /= 1000; i++ }
  return x.toFixed(x >= 100 ? 0 : x >= 10 ? 1 : 2) + ' ' + units[i]
}

export function formatDurationNs(ns: number | undefined): string {
  if (ns === undefined || ns === null) return '—'
  if (ns < 1_000) return ns + 'ns'
  if (ns < 1_000_000) return (ns / 1_000).toFixed(2) + 'µs'
  if (ns < 1_000_000_000) return (ns / 1_000_000).toFixed(2) + 'ms'
  return (ns / 1_000_000_000).toFixed(2) + 's'
}


// Convenience wrappers used in details UIs
export function bytesOrEmpty(v: number | undefined | null): string {
  const n = typeof v === 'number' ? v : 0
  return n > 0 ? humanBytes(n) : ''
}

export function pktsOrEmpty(v: number | undefined | null): string {
  const n = typeof v === 'number' ? v : 0
  return n > 0 ? humanPackets(n) + ' pkts' : ''
}
