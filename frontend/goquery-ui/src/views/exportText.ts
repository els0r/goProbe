import { FlowRecord } from '../api/domain'
import { humanBytes, humanPackets } from '../utils/format'
import { renderProto } from '../utils/proto'

function pad(s: string, w: number, align: 'left' | 'right' = 'left'): string {
  const str = s === undefined || s === null ? '' : String(s)
  if (str.length >= w) return align === 'left' ? str.slice(0, w) : str.slice(-w)
  const spaces = ' '.repeat(w - str.length)
  return align === 'left' ? str + spaces : spaces + str
}

function fmtDurationNsAsMsOrS(ns?: number): string {
  if (!ns || ns <= 0) return ''
  const ms = ns / 1_000_000
  return ms >= 1000 ? (ms / 1000).toFixed(2) + 's' : Math.round(ms) + 'ms'
}

export interface BuildTextOptions {
  attributes?: string[] | null
  totalsBytes?: number
  totalsPackets?: number
  meta?: {
    first?: string
    last?: string
    interfacesCount?: number
    hostsTotal?: number
    hostsOk?: number
    hostsErrors?: number
    sortBy?: 'bytes' | 'packets'
    hitsTotal?: number
    durationNs?: number
    br?: number
    bs?: number
    pr?: number
    ps?: number
  }
}

export function buildTextTable(rows: FlowRecord[], opts: BuildTextOptions = {}): string {
  const attributes = opts.attributes
  const showAll = !attributes || attributes.length === 0
  const show = (attr: string) => showAll || attributes.includes(attr)
  const anyHost = rows.some((r) => !!r.host)
  const anyIface = rows.some((r) => !!r.iface)

  const cols: Array<{ key: string; width: number; align: 'left' | 'right' }> = []
  if (anyHost) cols.push({ key: 'host', width: 22, align: 'right' })
  if (anyIface) cols.push({ key: 'iface', width: 8, align: 'right' })
  if (show('sip')) cols.push({ key: 'sip', width: 15, align: 'right' })
  if (show('dip')) cols.push({ key: 'dip', width: 15, align: 'right' })
  if (show('dport')) cols.push({ key: 'dport', width: 6, align: 'right' })
  if (show('proto')) cols.push({ key: 'proto', width: 6, align: 'right' })
  cols.push({ key: 'pin', width: 8, align: 'right' })
  cols.push({ key: 'pout', width: 8, align: 'right' })
  cols.push({ key: 'ppct', width: 6, align: 'right' })
  cols.push({ key: 'bin', width: 11, align: 'right' })
  cols.push({ key: 'bout', width: 11, align: 'right' })
  cols.push({ key: 'bpct', width: 6, align: 'right' })

  const spacerBetween = '  '
  const firstMetricIdx = cols.findIndex((c) => c.key === 'pin')
  const headerLine1Padding = cols
    .slice(0, firstMetricIdx)
    .reduce((s, c) => s + c.width + spacerBetween.length, 0)
  const header1 =
    ' '.repeat(Math.max(0, headerLine1Padding)) +
    pad('packets', cols[firstMetricIdx].width, 'right') +
    spacerBetween +
    pad('packets', cols[firstMetricIdx + 1].width, 'right') +
    spacerBetween +
    pad('', cols[firstMetricIdx + 2].width, 'right') +
    spacerBetween +
    pad('bytes', cols[firstMetricIdx + 3].width, 'right') +
    spacerBetween +
    pad('bytes', cols[firstMetricIdx + 4].width, 'right')

  const header2Labels: Record<string, string> = {
    host: 'host',
    iface: 'iface',
    sip: 'sip',
    dip: 'dip',
    dport: 'dport',
    proto: 'proto',
    pin: 'in',
    pout: 'out',
    ppct: '%',
    bin: 'in',
    bout: 'out',
    bpct: '%',
  }
  const header2 = cols
    .map((c) => pad(header2Labels[c.key] || c.key, c.width, c.align))
    .join(spacerBetween)

  const totalBytes = Math.max(0, Number(opts.totalsBytes) || 0)
  const totalPkts = Math.max(0, Number(opts.totalsPackets) || 0)
  const lines: string[] = []
  lines.push('')
  lines.push('  ' + header1)
  lines.push('  ' + header2)

  for (const r of rows) {
    const bt = (r.bytes_in || 0) + (r.bytes_out || 0)
    const pt = (r.packets_in || 0) + (r.packets_out || 0)
    const rowVals: Record<string, string> = {
      host: r.host || '',
      iface: r.iface || '',
      sip: r.sip || '',
      dip: r.dip || '',
      dport: r.dport === null || r.dport === undefined ? '' : String(r.dport),
      proto: renderProto(r.proto as any),
      pin: humanPackets(r.packets_in || 0),
      pout: humanPackets(r.packets_out || 0),
      ppct: totalPkts > 0 ? ((pt * 100) / totalPkts).toFixed(2) : '',
      bin: humanBytes(r.bytes_in || 0),
      bout: humanBytes(r.bytes_out || 0),
      bpct: totalBytes > 0 ? ((bt * 100) / totalBytes).toFixed(2) : '',
    }
    const line = cols.map((c) => pad(rowVals[c.key] || '', c.width, c.align)).join(spacerBetween)
    lines.push('  ' + line)
  }

  const totalHits = Number.isFinite(Number(opts?.meta?.hitsTotal))
    ? Number(opts?.meta?.hitsTotal)
    : undefined
  if (totalHits !== undefined && rows.length < totalHits) {
    const ellVals: Record<string, string> = {
      host: '',
      iface: '',
      sip: '',
      dip: '',
      dport: '',
      proto: '',
      pin: '...',
      pout: '...',
      ppct: '',
      bin: '...',
      bout: '...',
      bpct: '',
    }
    const ellLine = cols
      .map((c) => pad((ellVals as any)[c.key] || '', c.width, c.align))
      .join(spacerBetween)
    lines.push('  ' + ellLine)

    const tVals: Record<string, string> = {
      host: '',
      iface: '',
      sip: '',
      dip: '',
      dport: '',
      proto: '',
      pin: humanPackets(Math.max(0, Number(opts?.meta?.pr) || 0)),
      pout: humanPackets(Math.max(0, Number(opts?.meta?.ps) || 0)),
      ppct: '',
      bin: humanBytes(Math.max(0, Number(opts?.meta?.br) || 0)),
      bout: humanBytes(Math.max(0, Number(opts?.meta?.bs) || 0)),
      bpct: '',
    }
    const totLine = cols
      .map((c) => pad((tVals as any)[c.key] || '', c.width, c.align))
      .join(spacerBetween)
    lines.push('  ' + totLine)
  }

  const totPktsTxt = humanPackets(totalPkts)
  const totBytesTxt = humanBytes(totalBytes)
  lines.push('')
  {
    const bottomVals: Record<string, string> = {
      pin: '',
      pout: totPktsTxt,
      ppct: '',
      bin: '',
      bout: totBytesTxt,
      bpct: '',
      host: '',
      iface: '',
      sip: '',
      dip: '',
      dport: '',
      proto: '',
    }
    if (cols.length > 0) {
      bottomVals[cols[0].key] = 'Totals:'
    }
    const bottomLine = cols
      .map((c) => pad((bottomVals as any)[c.key] || '', c.width, c.align))
      .join(spacerBetween)
    lines.push('  ' + bottomLine)
  }

  const first = (() => {
    if (!opts?.meta?.first) return ''
    const d = new Date(opts.meta.first)
    if (isNaN(d.getTime())) return ''
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  })()
  const last = (() => {
    if (!opts?.meta?.last) return ''
    const d = new Date(opts.meta.last)
    if (isNaN(d.getTime())) return ''
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  })()
  const span = first && last ? `[${first}, ${last}]` : ''
  const durTxt = fmtDurationNsAsMsOrS(opts?.meta?.durationNs)
  const rangeTxt = (() => {
    const f = opts?.meta?.first ? new Date(opts.meta.first).getTime() : NaN
    const l = opts?.meta?.last ? new Date(opts.meta.last).getTime() : NaN
    if (!isFinite(f) || !isFinite(l)) return ''
    let ms = Math.max(0, l - f)
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
    ms -= s * secMs
    const parts: string[] = []
    if (d > 0) parts.push(d + 'd')
    if (h > 0 || d > 0) parts.push(h + 'h')
    if (m > 0 || (d === 0 && h === 0)) parts.push(m + 'm')
    if (d === 0 && h === 0 && m === 0) {
      if (s > 0) parts.push(s + 's')
      else parts.push(Math.round(ms) + 'ms')
    }
    return parts.join('')
  })()
  lines.push('')
  if (span) lines.push('Timespan           : ' + span + (rangeTxt ? ' ' + rangeTxt : ''))
  const ifacesTxt = (opts?.meta?.interfacesCount || 0) + ' queried'
  const hTotal = opts?.meta?.hostsTotal || 0
  const hOk = opts?.meta?.hostsOk || 0
  const hErr = opts?.meta?.hostsErrors || 0
  const hostsTxt = hErr > 0 ? `${hTotal} hosts: ${hOk} ok / ${hErr} errors` : `${hTotal} hosts`
  lines.push('Interfaces / Hosts : ' + ifacesTxt + ' on ' + hostsTxt)
  lines.push(
    'Sorted by          : ' +
      (opts?.meta?.sortBy === 'packets'
        ? 'packet count (sent and received)'
        : 'accumulated data volume (sent and received)')
  )
  const hits = totalHits !== undefined ? ` out of ${totalHits}` : ''
  lines.push(
    'Query stats        : displayed top ' +
      rows.length +
      ' hits' +
      hits +
      (durTxt ? ' in ' + durTxt : '')
  )

  lines.push('')
  return lines.join('\n')
}
