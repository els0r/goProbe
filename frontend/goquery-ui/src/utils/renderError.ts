export function renderError(e: unknown): string {
  if (!e) return ''
  if (typeof e === 'string') return e
  try {
    return String((e as any)?.message || JSON.stringify(e))
  } catch {
    return 'Error'
  }
}
