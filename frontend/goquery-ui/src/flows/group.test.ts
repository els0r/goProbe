import { describe, it, expect } from 'vitest'
import { inOutScaleMax } from './group'
import { FlowRecord } from './record'

// minimal FlowRecord carrying only the counters the helper reads
const flow = (p: Partial<FlowRecord>): FlowRecord => ({ ...p } as FlowRecord)

describe('inOutScaleMax', () => {
  it('returns 0 for an empty set', () => {
    expect(inOutScaleMax([], 'bytes_in', 'bytes_out')).toBe(0)
  })

  it('takes the max across both in and out over all rows', () => {
    const rows = [
      flow({ bytes_in: 10, bytes_out: 4 }),
      flow({ bytes_in: 3, bytes_out: 99 }),
      flow({ bytes_in: 7, bytes_out: 1 }),
    ]
    expect(inOutScaleMax(rows, 'bytes_in', 'bytes_out')).toBe(99)
  })

  it('reads the packet counters when asked', () => {
    const rows = [
      flow({ packets_in: 5, packets_out: 2 }),
      flow({ packets_in: 8, packets_out: 1 }),
    ]
    expect(inOutScaleMax(rows, 'packets_in', 'packets_out')).toBe(8)
  })

  it('treats absent counters as zero', () => {
    expect(inOutScaleMax([flow({ bytes_in: 42 })], 'bytes_in', 'bytes_out')).toBe(42)
  })
})
