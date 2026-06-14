/**
 * Geometry for one centered-axis diverging in/out bar.
 *
 * The axis sits at the horizontal centre of the track, so each direction owns
 * half of the available width. Both fractions are in `[0, 1]` and express how
 * much of that *half* the bar fills — `inFrac === 1` means the inbound (left)
 * bar reaches the far left edge.
 */
export interface InOutBarGeometry {
  inFrac: number
  outFrac: number
}

/**
 * Square-root–compressed fraction of `value` against a shared `scaleMax`.
 *
 * sqrt compression keeps the heavy tail of network traffic visible without
 * the zero-axis singularity or "half-bar = √value" deception of a log scale.
 * Non-positive or non-finite inputs collapse to `0`; the result is clamped to
 * `[0, 1]` so a transient streaming overshoot can never exceed the track.
 */
function scaledFrac(value: number, scaleMax: number): number {
  if (!(value > 0) || !(scaleMax > 0)) return 0
  const f = Math.sqrt(value) / Math.sqrt(scaleMax)
  return f > 1 ? 1 : f
}

/**
 * Compute the left/right fill fractions for a diverging in/out bar.
 *
 * Tolerates `undefined`/absent magnitudes and a missing/zero `scaleMax`,
 * returning a well-defined zero geometry — call sites can drop their guards.
 */
export function inOutBarGeometry(
  inValue: number | undefined | null,
  outValue: number | undefined | null,
  scaleMax: number | undefined | null,
): InOutBarGeometry {
  const max = typeof scaleMax === 'number' ? scaleMax : 0
  return {
    inFrac: scaledFrac(typeof inValue === 'number' ? inValue : 0, max),
    outFrac: scaledFrac(typeof outValue === 'number' ? outValue : 0, max),
  }
}
