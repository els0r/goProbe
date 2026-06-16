import { FlowRecord, runSharePct } from '../flows'
import { humanBytes, humanPackets } from '../utils/format'
import { renderProto } from '../utils/proto'
import { formatTimestamp, humanRangeDuration } from '../utils/timeFormat'

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

export interface BuildCsvOptions {
  // The attribute columns to render, in display order (same meaning as
  // BuildTextOptions.attributes). Empty/absent -> all canonical attributes.
  attributes?: string[] | null
}

// Canonical attribute columns, used when no explicit list is given. Mirrors
// ALLOWED_ATTR_ORDER; kept local so this pure formatter has no dependency on the
// AttributesSelect component module.
const CSV_DEFAULT_ATTRS = ['sip', 'dip', 'dport', 'proto']

function csvEscape(v: unknown): string {
  if (v === null || v === undefined) return ''
  let s = String(v)
  // Formula-injection guard (CWE-1236): a cell beginning with = + - @ TAB or CR
  // is evaluated as a formula by Excel/Sheets/LibreOffice. Flow values such as
  // reverse-DNS hostnames are attacker-influenced, so prefix the dangerous
  // leading character with a single quote to force literal text.
  if (/^[=+\-@\t\r]/.test(s)) s = "'" + s
  // RFC-4180 quoting: any field containing a comma, quote, CR or LF must be
  // wrapped in quotes (CR included so a stray \r can't split a row).
  if (/[",\n\r]/.test(s)) {
    return '"' + s.replace(/"/g, '""') + '"'
  }
  return s
}

// Pure rows -> RFC-4180 CSV string; the twin of buildTextTable. The caller keeps
// only the download plumbing (blob/anchor). Empty rows -> '' so the caller can
// skip the download. Columns: host?/iface? (only when any row carries one), the
// shown attributes (caller order, unlike buildTextTable's canonical order), then
// fixed per-row totals. bytes_total/packets_total are this row's in+out, NOT the
// Run aggregate.
export function buildCsv(rows: FlowRecord[], opts: BuildCsvOptions = {}): string {
  if (rows.length === 0) return ''
  const shown =
    !opts.attributes || opts.attributes.length === 0 ? CSV_DEFAULT_ATTRS : opts.attributes
  const anyHost = rows.some((r) => !!r.host)
  const anyIface = rows.some((r) => !!r.iface)
  const headers = [
    ...(anyHost ? ['host'] : []),
    ...(anyIface ? ['iface'] : []),
    ...shown,
    'bytes_in',
    'bytes_out',
    'bytes_total',
    'packets_in',
    'packets_out',
    'packets_total',
  ]
  const lines = [headers.join(',')]
  for (const r of rows) {
    const values: Array<string | number> = []
    if (anyHost) values.push(r.host || '')
    if (anyIface) values.push(r.iface || '')
    for (const a of shown) values.push((r as Record<string, any>)[a] ?? '')
    const bt = r.bytes_total
    const pt = r.packets_total
    values.push(r.bytes_in || 0, r.bytes_out || 0, bt, r.packets_in || 0, r.packets_out || 0, pt)
    lines.push(values.map(csvEscape).join(','))
  }
  return lines.join('\n')
}

export function buildTextTable(rows: FlowRecord[], opts: BuildTextOptions = {}): string {
  const attributes = opts.attributes
  const showAll = !attributes || attributes.length === 0
  const show = (attr: string) => showAll || attributes.includes(attr)
  const anyHost = rows.some((r) => !!r.host)
  const anyIface = rows.some((r) => !!r.iface)

  const totalBytes = Math.max(0, Number(opts.totalsBytes) || 0)
  const totalPkts = Math.max(0, Number(opts.totalsPackets) || 0)

  // Pre-compute display values for all rows so column widths can be measured
  const rowVals: Record<string, string>[] = rows.map((r) => {
    const bt = r.bytes_total
    const pt = r.packets_total
    return {
      host: r.host || '',
      iface: r.iface || '',
      sip: r.sip || '',
      dip: r.dip || '',
      dport: r.dport === null || r.dport === undefined ? '' : String(r.dport),
      proto: renderProto(r.proto as any),
      pin: humanPackets(r.packets_in || 0),
      pout: humanPackets(r.packets_out || 0),
      ppct: totalPkts > 0 ? runSharePct(pt, totalPkts).toFixed(2) : '',
      bin: humanBytes(r.bytes_in || 0),
      bout: humanBytes(r.bytes_out || 0),
      bpct: totalBytes > 0 ? runSharePct(bt, totalBytes).toFixed(2) : '',
    }
  })

  // Compute width as max of header label length and all data value lengths
  const dynWidth = (key: string, headerLabel: string, minW: number): number => {
    const dataMax = rowVals.reduce((m, r) => Math.max(m, (r[key] || '').length), 0)
    return Math.max(minW, headerLabel.length, dataMax)
  }

  const cols: Array<{ key: string; width: number; align: 'left' | 'right' }> = []
  if (anyHost) cols.push({ key: 'host', width: dynWidth('host', 'host', 6), align: 'right' })
  if (anyIface) cols.push({ key: 'iface', width: dynWidth('iface', 'iface', 5), align: 'right' })
  if (show('sip')) cols.push({ key: 'sip', width: dynWidth('sip', 'sip', 7), align: 'right' })
  if (show('dip')) cols.push({ key: 'dip', width: dynWidth('dip', 'dip', 7), align: 'right' })
  if (show('dport')) cols.push({ key: 'dport', width: dynWidth('dport', 'dport', 5), align: 'right' })
  if (show('proto')) cols.push({ key: 'proto', width: dynWidth('proto', 'proto', 5), align: 'right' })
  cols.push({ key: 'pin', width: dynWidth('pin', 'in', 6), align: 'right' })
  cols.push({ key: 'pout', width: dynWidth('pout', 'out', 6), align: 'right' })
  cols.push({ key: 'ppct', width: dynWidth('ppct', '%', 5), align: 'right' })
  cols.push({ key: 'bin', width: dynWidth('bin', 'in', 6), align: 'right' })
  cols.push({ key: 'bout', width: dynWidth('bout', 'out', 6), align: 'right' })
  cols.push({ key: 'bpct', width: dynWidth('bpct', '%', 5), align: 'right' })

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

  const lines: string[] = []
  lines.push('')
  lines.push('  ' + header1)
  lines.push('  ' + header2)

  for (const rv of rowVals) {
    const line = cols.map((c) => pad(rv[c.key] || '', c.width, c.align)).join(spacerBetween)
    lines.push('  ' + line)
  }

  const totalHits = Number.isFinite(Number(opts?.meta?.hitsTotal))
    ? Number(opts?.meta?.hitsTotal)
    : undefined
  if (totalHits !== undefined && rows.length < totalHits) {
    const ellVals: Record<string, string> = {
      host: '', iface: '', sip: '', dip: '', dport: '', proto: '',
      pin: '...', pout: '...', ppct: '', bin: '...', bout: '...', bpct: '',
    }
    lines.push('  ' + cols.map((c) => pad(ellVals[c.key] || '', c.width, c.align)).join(spacerBetween))

    const tVals: Record<string, string> = {
      host: '', iface: '', sip: '', dip: '', dport: '', proto: '',
      pin: humanPackets(Math.max(0, Number(opts?.meta?.pr) || 0)),
      pout: humanPackets(Math.max(0, Number(opts?.meta?.ps) || 0)),
      ppct: '',
      bin: humanBytes(Math.max(0, Number(opts?.meta?.br) || 0)),
      bout: humanBytes(Math.max(0, Number(opts?.meta?.bs) || 0)),
      bpct: '',
    }
    lines.push('  ' + cols.map((c) => pad(tVals[c.key] || '', c.width, c.align)).join(spacerBetween))
  }

  const totBytesTxt = humanBytes(totalBytes)
  const totPktsTxt = humanPackets(totalPkts)
  lines.push('')
  const bottomVals: Record<string, string> = {
    host: '', iface: '', sip: '', dip: '', dport: '', proto: '',
    pin: '', pout: totPktsTxt, ppct: '', bin: '', bout: totBytesTxt, bpct: '',
  }
  if (cols.length > 0) bottomVals[cols[0].key] = 'Totals:'
  lines.push('  ' + cols.map((c) => pad(bottomVals[c.key] || '', c.width, c.align)).join(spacerBetween))

  const first = formatTimestamp(opts?.meta?.first)
  const last = formatTimestamp(opts?.meta?.last)
  const span = opts?.meta?.first && opts?.meta?.last ? `[${first}, ${last}]` : ''
  const rangeTxt = humanRangeDuration(opts?.meta?.first, opts?.meta?.last)
  const durTxt = fmtDurationNsAsMsOrS(opts?.meta?.durationNs)

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
