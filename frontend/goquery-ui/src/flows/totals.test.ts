import { describe, it, expect } from 'vitest'
import { resultTotals, runSharePct } from './totals'
import { SummarySchema } from '../api/domain'

// minimal Summary carrying only the totals field the function reads
const withTotals = (totals: Record<string, number>): SummarySchema =>
  ({ totals } as unknown as SummarySchema)

describe('resultTotals', () => {
  it('returns all-zero totals when the summary is undefined', () => {
    expect(resultTotals(undefined)).toEqual({
      bytes_in: 0,
      bytes_out: 0,
      bytes_total: 0,
      packets_in: 0,
      packets_out: 0,
      packets_total: 0,
    })
  })

  it('treats absent counters as zero and still derives the total', () => {
    expect(resultTotals(withTotals({ br: 100 }))).toEqual({
      bytes_in: 100,
      bytes_out: 0,
      bytes_total: 100,
      packets_in: 0,
      packets_out: 0,
      packets_total: 0,
    })
  })

  it('decodes br/bs/pr/ps and derives in+out totals', () => {
    expect(resultTotals(withTotals({ br: 30, bs: 12, pr: 5, ps: 3 }))).toEqual({
      bytes_in: 30,
      bytes_out: 12,
      bytes_total: 42,
      packets_in: 5,
      packets_out: 3,
      packets_total: 8,
    })
  })
})

describe('runSharePct', () => {
  it('returns 0 when the Run Total is zero, negative, or NaN', () => {
    expect(runSharePct(50, 0)).toBe(0)
    expect(runSharePct(50, -10)).toBe(0)
    expect(runSharePct(50, NaN)).toBe(0)
  })

  it('returns the part as a percentage of the Run Total', () => {
    expect(runSharePct(25, 100)).toBe(25)
    expect(runSharePct(1, 8)).toBeCloseTo(12.5)
  })

  it('clamps a part exceeding the Run Total to 100 (streaming overshoot)', () => {
    expect(runSharePct(150, 100)).toBe(100)
  })

  it('clamps a negative part to 0', () => {
    expect(runSharePct(-5, 100)).toBe(0)
  })
})
