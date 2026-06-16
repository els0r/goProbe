import { useState, useRef, useEffect } from 'react'
import { Chevron } from './Chevron'

export interface AttributeOption {
  label: string
  value: string
}

export interface AttributePreset {
  label: string
  values: string[]
  all: boolean
}

const ALL_VALUE = '*'

export interface AttributesSelectProps {
  options: AttributeOption[]
  value: string[] // explicit list of selected attributes; empty or full == all
  allSelected: boolean // kept for external state compatibility
  onChange: (next: { values: string[]; all: boolean }) => void
  hasError?: boolean
  /** Open the dropdown upward instead of downward */
  dropUp?: boolean
  /** Quick-select presets rendered below the attribute checkboxes */
  presets?: AttributePreset[]
}

// Multi-select dropdown without "All" entry; empty or full selection treated as all.
export function AttributesSelect({
  options,
  value,
  allSelected,
  onChange,
  hasError,
  dropUp,
  presets,
}: AttributesSelectProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!ref.current) return
      if (!ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  function toggle(key: string) {
    const canonical = ATTR_ALIASES[key] || key
    let base = allSelected ? [] : value.map((v) => ATTR_ALIASES[v] || v)
    const exists = base.includes(canonical)
    const next = exists ? base.filter((v) => v !== canonical) : [...base, canonical]
    if (next.length === 0 || next.length === options.length) {
      onChange({ values: [], all: true })
      return
    }
    onChange({ values: next, all: false })
  }

  const label = allSelected
    ? 'All'
    : value.length === 0
      ? 'All'
      : options
        .filter((o) => value.includes(o.value))
        .map((o) => o.label)
        .join(', ')

  return (
    <div className="relative w-full" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={
          `w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] text-left ring-1 flex items-center justify-between ` +
          (hasError
            ? 'ring-red-500/40 bg-red-500/10 focus:outline-none focus:ring-red-500/40'
            : 'ring-line focus:outline-none focus:ring-primary-500')
        }
      >
        <span className="truncate">{label}</span>
        <Chevron open={open} className="ml-1" />
      </button>
      {open && (
        <div className={`absolute z-20 w-full rounded-md border border-line bg-surface-100 p-1 shadow-lg ${dropUp ? 'bottom-full mb-1' : 'mt-1'}`}>
          <ul className="max-h-60 overflow-auto scroll-thin text-xs">
            {options.map((o) => {
              const canonical = ATTR_ALIASES[o.value] || o.value
              const checked = allSelected
                ? true
                : value.some((v) => (ATTR_ALIASES[v] || v) === canonical)
              return (
                <li key={o.value}>
                  <label className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 hover:bg-surface-200">
                    <input type="checkbox" checked={checked} onChange={() => toggle(o.value)} />
                    <span>{o.label}</span>
                  </label>
                </li>
              )
            })}
          </ul>
          {presets && presets.length > 0 && (
            <>
              <div className="my-1 border-t border-line" />
              <ul className="text-xs">
                {presets.map((p) => {
                  const active = allSelected
                    ? p.all
                    : !p.all && p.values.join(',') === value.join(',')
                  return (
                    <li key={p.label}>
                      <button
                        type="button"
                        onClick={() => {
                          onChange(
                            p.all
                              ? { values: [], all: true }
                              : { values: p.values, all: false },
                          )
                          setOpen(false)
                        }}
                        className={`w-full rounded px-2 py-1 text-left ${active
                          ? 'bg-primary-600 text-on-accent'
                          : 'hover:bg-surface-200'
                          }`}
                      >
                        {p.label}
                      </button>
                    </li>
                  )
                })}
              </ul>
            </>
          )}
        </div>
      )}
    </div>
  )
}

// allowed attribute keys (defensive whitelist)
export const ALLOWED_ATTR_ORDER: string[] = ['sip', 'dip', 'dport', 'proto']
const ALLOWED_ATTR = new Set(ALLOWED_ATTR_ORDER)
// alias map to normalize different UI labels/inputs -> canonical keys
const ATTR_ALIASES: Record<string, string> = {
  protocol: 'proto',
  port: 'dport',
}

export function parseAttributeQuery(q: string | undefined): {
  values: string[]
  all: boolean
} {
  if (!q) return { values: Array.from(ALLOWED_ATTR), all: true } // empty -> all
  if (q === ALL_VALUE) return { values: Array.from(ALLOWED_ATTR), all: true }
  const parts = q
    .split(',')
    .map((s) => s.trim())
    .map((p) => ATTR_ALIASES[p] || p)
    .filter((p) => p && p !== 'undefined' && ALLOWED_ATTR.has(p))
  if (parts.length === 0) return { values: Array.from(ALLOWED_ATTR), all: true }
  // dedupe preserving order
  const seen = new Set<string>()
  const dedup: string[] = []
  for (const p of parts) {
    if (!seen.has(p)) {
      seen.add(p)
      dedup.push(p)
    }
  }
  if (dedup.length === ALLOWED_ATTR.size) return { values: dedup, all: true }
  return { values: dedup, all: false }
}

export function buildAttributeQuery(values: string[], all: boolean): string {
  if (all) {
    // All selected -> send explicit full list so backend validation (min length) passes
    return ALLOWED_ATTR_ORDER.join(',')
  }
  const filtered = values
    .map((v) => ATTR_ALIASES[v] || v)
    .filter((v) => v && v !== 'undefined' && ALLOWED_ATTR.has(v))
  if (!filtered.length) return ''
  // maintain canonical order in output
  const ordered = ALLOWED_ATTR_ORDER.filter((a) => filtered.includes(a))
  return ordered.join(',')
}

// The attribute columns to display for a given selection state. `all` -> every
// allowed attribute in canonical order; otherwise the selected subset in caller
// order. parseAttributeQuery already canonicalizes/filters `values`, so no alias
// mapping is needed here. Returns a fresh array so callers can mutate it without
// corrupting the shared ALLOWED_ATTR_ORDER or the caller's state.
export function shownAttributes(attr: { values: string[]; all: boolean }): string[] {
  return attr.all ? [...ALLOWED_ATTR_ORDER] : [...attr.values]
}
