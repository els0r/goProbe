import protoMapRaw from '../api/proto-map.json'

// build a number->name lookup once from the generated JSON
const protoLookup: Record<number, string> = (() => {
  const out: Record<number, string> = {}
  const src: any = (protoMapRaw as any) || {}
  if (src && typeof src === 'object') {
    for (const k of Object.keys(src)) {
      const num = Number(k)
      if (Number.isNaN(num)) continue
      const entry: any = (src as any)[k]
      if (!entry || typeof entry.text !== 'string') continue
      const text = entry.text.trim()
      if (!text) continue
      out[num] = text
    }
  }
  return out
})()

export function renderProto(num: number | null | undefined): string {
  if (num === null || num === undefined) return '—'
  const name = protoLookup[num]
  return name ? `${name}` : String(num)
}

export function getProtoName(num: number | null | undefined): string | undefined {
  if (num === null || num === undefined) return undefined
  return protoLookup[num]
}
