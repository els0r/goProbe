import { describe, it, expect } from 'vitest'
import { inOutBarGeometry } from './inOutBar'

describe('inOutBarGeometry', () => {
  it('returns zero geometry when scaleMax is missing or zero', () => {
    expect(inOutBarGeometry(100, 200, 0)).toEqual({ inFrac: 0, outFrac: 0 })
    expect(inOutBarGeometry(100, 200, undefined)).toEqual({ inFrac: 0, outFrac: 0 })
  })

  it('treats absent / non-positive magnitudes as an empty half', () => {
    expect(inOutBarGeometry(undefined, undefined, 100)).toEqual({ inFrac: 0, outFrac: 0 })
    // unidirectional: only the outbound half draws
    expect(inOutBarGeometry(0, 100, 100)).toEqual({ inFrac: 0, outFrac: 1 })
  })

  it('fills the half when the value equals the scale max', () => {
    expect(inOutBarGeometry(100, 100, 100)).toEqual({ inFrac: 1, outFrac: 1 })
  })

  it('compresses with a square-root curve, not linearly', () => {
    // a value at 1/4 of the max fills 1/2 the half (sqrt), not 1/4
    const g = inOutBarGeometry(25, 100, 100)
    expect(g.inFrac).toBeCloseTo(0.5, 10)
    expect(g.outFrac).toBe(1)
  })

  it('clamps a value above the scale max to a full half', () => {
    // a streaming overshoot must never exceed the track
    expect(inOutBarGeometry(400, 0, 100)).toEqual({ inFrac: 1, outFrac: 0 })
  })
})
