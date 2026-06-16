import { describe, it, expect } from 'vitest'
import { buildGraph, edgeWidth01 } from './model'
import type { FlowRecord } from '../../flows'

// minimal FlowRecord factory — only the fields buildGraph reads matter
function mkFlow(o: Partial<FlowRecord> & { sip: string; dip: string }): FlowRecord {
  const bytes_in = o.bytes_in ?? 0
  const bytes_out = o.bytes_out ?? 0
  const packets_in = o.packets_in ?? 0
  const packets_out = o.packets_out ?? 0
  return {
    sip: o.sip,
    dip: o.dip,
    dport: o.dport ?? null,
    proto: o.proto ?? null,
    bytes_in,
    bytes_out,
    bytes_total: o.bytes_total ?? bytes_in + bytes_out,
    packets_in,
    packets_out,
    packets_total: o.packets_total ?? packets_in + packets_out,
    host: o.host,
    host_id: o.host_id,
    iface: o.iface,
    interval_end: o.interval_end,
    bidirectional:
      o.bidirectional ?? (bytes_in > 0 && bytes_out > 0 && packets_in > 0 && packets_out > 0),
    _raw: o._raw ?? ({} as FlowRecord['_raw']),
  }
}

const big = 1024 // generous node cap so the budget never bites unless intended

describe('buildGraph — empty input', () => {
  it('returns an empty, non-throwing model for no rows', () => {
    const m = buildGraph([], { maxNodes: big })
    expect(m.nodes).toEqual([])
    expect(m.edges).toEqual([])
    expect(m.ipTotals.size).toBe(0)
    expect(m.hostTotals.size).toBe(0)
    expect(m.droppedForCap).toBe(0)
  })
})

describe('buildGraph — nodes and node-count budget', () => {
  it('creates an ip/iface/host node per distinct entity', () => {
    const m = buildGraph([mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 10 })], {
      maxNodes: big,
    })
    const ids = new Set(m.nodes.map((n) => n.id))
    expect(ids).toEqual(new Set(['ip:A', 'ip:B', 'iface:h1:eth0', 'host:h1']))
    expect(m.droppedForCap).toBe(0)
  })

  it('drops whole flows once the node cap is reached, never exceeding maxNodes', () => {
    const flows = [
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 100 }), // 4 nodes
      mkFlow({ sip: 'C', dip: 'D', host: 'h2', iface: 'eth1', bytes_total: 50 }), // +4 nodes
    ]
    const m = buildGraph(flows, { maxNodes: 4 })
    expect(m.nodes.length).toBeLessThanOrEqual(4)
    expect(m.droppedForCap).toBe(1)
  })

  it('counts the first flow as included but still caps its nodes per-node', () => {
    // The first flow bypasses the break (included.length === 0), so it is never
    // "dropped for cap" — but ensureNode independently enforces the cap, so a
    // maxNodes=1 budget yields a single node and edges that reference absent ones.
    const m = buildGraph([mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0' })], { maxNodes: 1 })
    expect(m.nodes.length).toBe(1)
    expect(m.droppedForCap).toBe(0)
  })

  it('prefers heavier flows: sorts by bytes_total before applying the budget', () => {
    const flows = [
      mkFlow({ sip: 'light', dip: 'lightD', host: 'hl', iface: 'e0', bytes_total: 1 }),
      mkFlow({ sip: 'heavy', dip: 'heavyD', host: 'hh', iface: 'e1', bytes_total: 9999 }),
    ]
    const m = buildGraph(flows, { maxNodes: 4 })
    const ids = new Set(m.nodes.map((n) => n.id))
    expect(ids.has('ip:heavy')).toBe(true)
    expect(ids.has('ip:light')).toBe(false)
    expect(m.droppedForCap).toBe(1)
  })
})

describe('buildGraph — edge aggregation', () => {
  it('emits two edges per flow (sip→iface, iface→dip)', () => {
    const m = buildGraph([mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 10 })], {
      maxNodes: big,
    })
    expect(m.edges).toHaveLength(2)
    expect(m.edges.map((e) => `${e.from}->${e.to}`).sort()).toEqual([
      'iface:h1:eth0->ip:B',
      'ip:A->iface:h1:eth0',
    ])
  })

  it('merges same-direction same-category edges, summing totals', () => {
    const flows = [
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 30, packets_total: 3 }),
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 70, packets_total: 7 }),
    ]
    const m = buildGraph(flows, { maxNodes: big })
    expect(m.edges).toHaveLength(2) // still just sip→iface and iface→dip
    for (const e of m.edges) {
      expect(e.bytesTotal).toBe(100)
      expect(e.packetsTotal).toBe(10)
    }
  })

  it('keeps bidirectional and unidirectional edges in separate buckets', () => {
    const flows = [
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_in: 5, bytes_out: 5, packets_in: 1, packets_out: 1 }), // bi
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_in: 5, bytes_out: 0, packets_in: 1, packets_out: 0 }), // uni
    ]
    const m = buildGraph(flows, { maxNodes: big })
    expect(m.edges).toHaveLength(4) // 2 directions × 2 categories
    expect(m.edges.filter((e) => e.dircat === 'bi')).toHaveLength(2)
    expect(m.edges.filter((e) => e.dircat === 'uni')).toHaveLength(2)
  })

  it('merges sip/dip membership uniquely across aggregated edges', () => {
    const flows = [
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 10 }),
      mkFlow({ sip: 'C', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 10 }),
    ]
    const m = buildGraph(flows, { maxNodes: big })
    // the iface→dip edge to B carries both source IPs that reached it
    const toB = m.edges.find((e) => e.to === 'ip:B')!
    expect(new Set(toB.sips)).toEqual(new Set(['A', 'C']))
    expect(toB.dips).toEqual(['B'])
  })
})

describe('buildGraph — totals and dircat', () => {
  it('accumulates per-IP and per-host totals across flows', () => {
    const flows = [
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 100, packets_total: 4 }),
      mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_total: 50, packets_total: 2 }),
    ]
    const m = buildGraph(flows, { maxNodes: big })
    expect(m.ipTotals.get('A')).toEqual({ bytes: 150, packets: 6 })
    expect(m.ipTotals.get('B')).toEqual({ bytes: 150, packets: 6 })
    expect(m.hostTotals.get('h1')).toEqual({ bytes: 150, packets: 6 })
  })

  it('derives dircat from the flow bidirectional flag', () => {
    const m = buildGraph(
      [mkFlow({ sip: 'A', dip: 'B', host: 'h1', iface: 'eth0', bytes_in: 1, bytes_out: 1, packets_in: 1, packets_out: 1 })],
      { maxNodes: big }
    )
    expect(m.edges.every((e) => e.dircat === 'bi')).toBe(true)
  })
})

describe('edgeWidth01', () => {
  it('floors non-positive or non-finite input to width 1', () => {
    expect(edgeWidth01(0)).toBe(1)
    expect(edgeWidth01(-1)).toBe(1)
    expect(edgeWidth01(NaN)).toBe(1)
    expect(edgeWidth01(Infinity)).toBe(1)
  })

  it('buckets 0..1 into stroke widths 1..8', () => {
    expect(edgeWidth01(0.01)).toBe(1)
    expect(edgeWidth01(0.5)).toBe(5)
    expect(edgeWidth01(1)).toBe(8)
  })
})
