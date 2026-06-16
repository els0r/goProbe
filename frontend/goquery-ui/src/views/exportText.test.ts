import { describe, it, expect } from 'vitest'
import { buildCsv } from './exportText'
import { FlowRecord } from '../flows'

// Minimal FlowRecord factory; tests override only the fields they care about.
const rec = (over: Partial<FlowRecord> = {}): FlowRecord => {
  const r = {
    sip: '1.1.1.1',
    dip: '2.2.2.2',
    dport: 443,
    proto: 6,
    bytes_in: 10,
    bytes_out: 5,
    packets_in: 3,
    packets_out: 2,
    bidirectional: true,
    _raw: {} as FlowRecord['_raw'],
    ...over,
  }
  // derive totals from the merged counters, mirroring flattenRow
  return { ...r, bytes_total: r.bytes_in + r.bytes_out, packets_total: r.packets_in + r.packets_out }
}

describe('buildCsv', () => {
  it('returns an empty string for no rows (caller skips the download)', () => {
    expect(buildCsv([])).toBe('')
  })

  it('emits a header plus one line per row with per-row in+out totals', () => {
    const csv = buildCsv([rec()], { attributes: ['sip', 'dip', 'dport', 'proto'] })
    const [header, row] = csv.split('\n')
    expect(header).toBe(
      'sip,dip,dport,proto,bytes_in,bytes_out,bytes_total,packets_in,packets_out,packets_total'
    )
    // bytes_total = 10+5, packets_total = 3+2 (per row, not the Run aggregate)
    expect(row).toBe('1.1.1.1,2.2.2.2,443,6,10,5,15,3,2,5')
  })

  it('RFC-4180 escapes values containing comma, quote, or newline', () => {
    const csv = buildCsv([rec({ host: 'a,b' }), rec({ host: 'c"d' }), rec({ host: 'e\nf' })])
    const lines = csv.split('\n')
    expect(lines[0].startsWith('host,')).toBe(true)
    expect(lines[1].startsWith('"a,b",')).toBe(true)
    expect(lines[2].startsWith('"c""d",')).toBe(true)
    // an embedded newline stays inside the quoted field, so the record spans
    // the raw-split boundary
    expect(csv).toContain('"e\nf"')
  })

  it('neutralizes spreadsheet formula injection by prefixing dangerous leads', () => {
    // A host carrying an attacker-influenced reverse-DNS name must not be
    // evaluated as a formula when the CSV is opened in Excel/Sheets.
    const lines = buildCsv([
      rec({ host: '=cmd|"/c calc"!A1' }),
      rec({ host: '+1+1' }),
      rec({ host: '-2+3' }),
      rec({ host: '@SUM(A1)' }),
    ]).split('\n')
    // '=...' also contains a comma/quote so it is single-quoted then RFC-4180 quoted
    expect(lines[1].startsWith('"\'=cmd')).toBe(true)
    expect(lines[2].startsWith("'+1+1,")).toBe(true)
    expect(lines[3].startsWith("'-2+3,")).toBe(true)
    expect(lines[4].startsWith("'@SUM(A1),")).toBe(true)
  })

  it('leaves safe values (IPs, ports, counters) untouched', () => {
    // A leading digit or letter is not a formula trigger, so no prefix is added.
    const row = buildCsv([rec({ host: 'host-1.example.com' })]).split('\n')[1]
    expect(row.startsWith('host-1.example.com,')).toBe(true)
  })

  it('includes host/iface columns only when some row carries them', () => {
    expect(buildCsv([rec()]).split('\n')[0]).not.toContain('host')
    expect(buildCsv([rec({ host: 'h1' })]).split('\n')[0].startsWith('host,')).toBe(true)
    expect(buildCsv([rec({ iface: 'eth0' })]).split('\n')[0].startsWith('iface,')).toBe(true)
  })

  it('renders the given attribute subset in caller order', () => {
    const header = buildCsv([rec()], { attributes: ['dip', 'sip'] }).split('\n')[0]
    expect(header).toBe('dip,sip,bytes_in,bytes_out,bytes_total,packets_in,packets_out,packets_total')
  })

  it('defaults to all canonical attributes when none are given', () => {
    const header = buildCsv([rec()]).split('\n')[0]
    expect(header).toBe(
      'sip,dip,dport,proto,bytes_in,bytes_out,bytes_total,packets_in,packets_out,packets_total'
    )
  })

  it('renders a null dport as an empty cell', () => {
    const row = buildCsv([rec({ dport: null })], { attributes: ['dport'] }).split('\n')[1]
    expect(row).toBe(',10,5,15,3,2,5')
  })
})
