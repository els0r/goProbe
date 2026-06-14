// Theme resolution layer.
//
// Single, framework-agnostic source of theme truth shared by the pre-paint
// FOUC-guard script (which re-implements resolveTheme inline because it runs
// before the bundle loads) and the React layer. Keep the localStorage key in
// sync with the literal used in index.html.

export type ThemePreference = 'system' | 'light' | 'dark'

export const LS_THEME_KEY = 'goquery_ui_theme'

const PREFERENCES: ReadonlyArray<ThemePreference> = ['system', 'light', 'dark']

/** Read the stored preference, validating it and defaulting to 'system'. */
export function readStoredPreference(): ThemePreference {
  try {
    const saved = localStorage.getItem(LS_THEME_KEY)
    if (saved && PREFERENCES.includes(saved as ThemePreference)) {
      return saved as ThemePreference
    }
  } catch {}
  return 'system'
}

/** Resolve a preference to a concrete theme, consulting the OS for 'system'. */
export function resolveTheme(pref: ThemePreference): 'light' | 'dark' {
  if (pref === 'light' || pref === 'dark') return pref
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** Apply the resolved theme to the document root. */
export function applyTheme(pref: ThemePreference): void {
  const resolved = resolveTheme(pref)
  document.documentElement.setAttribute('data-theme', resolved)
  document.documentElement.style.colorScheme = resolved
}

/** Persist the preference. */
export function storePreference(pref: ThemePreference): void {
  try {
    localStorage.setItem(LS_THEME_KEY, pref)
  } catch {}
}

/**
 * Subscribe to OS colour-scheme changes. Returns an unsubscribe function.
 * Callers should only re-apply when the active preference is 'system'.
 */
export function watchSystemTheme(onChange: () => void): () => void {
  const mql = window.matchMedia('(prefers-color-scheme: dark)')
  mql.addEventListener('change', onChange)
  return () => mql.removeEventListener('change', onChange)
}
