// errorText.ts — text helpers for surfacing validation errors: render a value
// for display, and normalize a message for matching. Consumed by the error-
// presentation cluster (errorMapping.ts, ErrorBanner). Awaiting its own error/
// home (ADR-0003); formerly the error half of utils/inputSanitize.ts.

export function formatValue(v: unknown): string {
  if (v === null) return 'null'
  if (v === undefined) return 'undefined'
  if (typeof v === 'string') return JSON.stringify(v)
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  try {
    return JSON.stringify(v)
  } catch {
    return '[unserializable]'
  }
}

// normalize strings for error matching: lower-case and unify curly apostrophes
export function normalizeText(s: string | undefined | null): string {
  if (!s) return ''
  return String(s)
    .toLowerCase()
    .replace(/\u2019/g, "'")
    .trim()
}
