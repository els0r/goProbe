import { describe, it, expect } from 'vitest'
import { buildGraph } from './model'
import { layoutGraph, IP_R } from './layout'
import type { FlowRecord } from '../../flows'

function mkFlow(o: Partial<FlowRecord> & { sip: string; dip: string }): FlowRecord {
  const bytes_in = o.bytes_in ?? 0
  const bytes_out = o.bytes_out ?? 0
  return {
    sip: o.sip,
    dip: o.dip,
    dport: null,
    proto: null,
    bytes_in,
    bytes_out,
    bytes_total: o.bytes_total ?? bytes_in + bytes_out,
    packets_in: 0,
    packets_out: 0,
    packets_total: o.packets_total ?? 0,
    host: o.host,
    host_id: o.host_id,
    iface: o.iface,
    interval_end: o.interval_end,
    bidirectional: o.bidirectional ?? false,
    _raw: ({} as FlowRecord['_raw']),
  }
}

const big = 1024
const MIN_IP_GAP = 2 * IP_R + 8 // mirror layout's intra-group spacing

// N public source IPs all talking to one public dest, over one host/iface.
function fanIn(n: number): FlowRecord[] {
  return Array.from({ length: n }, (_, i) =>
    mkFlow({ sip: `8.8.8.${i + 1}`, dip: '9.9.9.9', host: 'h1', iface: 'eth0', bytes_total: 100 - i })
  )
}

describe('layoutGraph — placement', () => {
  it('puts source IPs left, dest IPs right, host in the middle column', () => {
    const model = buildGraph(fanIn(3), { maxNodes: big })
    const { nodes } = layoutGraph(model, { width: 800, height: 600 })
    const byId = new Map(nodes.map((n) => [n.id, n]))
    expect(byId.get('ip:8.8.8.1')!.x).toBeLessThan(400)
    expect(byId.get('ip:9.9.9.9')!.x).toBeGreaterThan(400)
    expect(byId.get('host:h1')!.x).toBe(400)
  })

  it('assigns a radius to host bubbles only', () => {
    const model = buildGraph(fanIn(2), { maxNodes: big })
    const { nodes } = layoutGraph(model, { width: 800, height: 600 })
    const host = nodes.find((n) => n.type === 'host')!
    const ip = nodes.find((n) => n.type === 'ip')!
    expect(host.radius).toBeGreaterThan(0)
    expect(ip.radius).toBeUndefined()
  })

  it('keeps at least MIN_IP_GAP between stacked IPs in a column', () => {
    const model = buildGraph(fanIn(4), { maxNodes: big })
    const { nodes } = layoutGraph(model, { width: 800, height: 600 })
    const leftYs = nodes
      .filter((n) => n.type === 'ip' && n.x < 400)
      .map((n) => n.y)
      .sort((a, b) => a - b)
    for (let i = 1; i < leftYs.length; i++) {
      expect(leftYs[i] - leftYs[i - 1]).toBeGreaterThanOrEqual(MIN_IP_GAP - 1e-6)
    }
  })
})

describe('layoutGraph — spatial budget', () => {
  it('does not prune when everything fits', () => {
    const model = buildGraph(fanIn(3), { maxNodes: big })
    const out = layoutGraph(model, { width: 800, height: 600 })
    expect(out.droppedForSpace).toBe(0)
    expect(out.nodes.filter((n) => n.type === 'ip')).toHaveLength(4) // 3 sources + 1 dest
  })

  it('prunes the lowest-volume IPs that cannot fit and reports the count', () => {
    const model = buildGraph(fanIn(10), { maxNodes: big })
    // span = 200 - 2*40 = 120 -> maxFit = floor(120/40)+1 = 4 left nodes
    const out = layoutGraph(model, { width: 800, height: 200 })
    const leftKept = out.nodes.filter((n) => n.type === 'ip' && n.x < 400)
    expect(leftKept.length).toBe(4)
    expect(out.droppedForSpace).toBe(6) // 10 sources -> 4 kept
    // heaviest source survives, lightest is pruned (sorted by volume desc)
    const ids = new Set(out.nodes.map((n) => n.id))
    expect(ids.has('ip:8.8.8.1')).toBe(true) // bytes_total 100
    expect(ids.has('ip:8.8.8.10')).toBe(false) // bytes_total 91 (tail)
  })

  it('removes edges whose endpoints were pruned', () => {
    const model = buildGraph(fanIn(10), { maxNodes: big })
    const out = layoutGraph(model, { width: 800, height: 200 })
    const present = new Set(out.nodes.map((n) => n.id))
    for (const e of out.edges) {
      expect(present.has(e.from)).toBe(true)
      expect(present.has(e.to)).toBe(true)
    }
  })
})

describe('layoutGraph — purity', () => {
  it('never mutates the geometry-free model', () => {
    const model = buildGraph(fanIn(3), { maxNodes: big })
    layoutGraph(model, { width: 800, height: 600 })
    for (const n of model.nodes) {
      const geom = n as unknown as { x?: number; y?: number }
      expect(geom.x).toBeUndefined()
      expect(geom.y).toBeUndefined()
    }
  })
})
