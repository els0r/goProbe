import { useState, useRef, useEffect } from 'react'

export interface AttributeOption {
  label: string
  value: string
}

const ALL_VALUE = '*'

export interface AttributesSelectProps {
  options: AttributeOption[]
  value: string[] // explicit list of selected attributes; empty or full == all
  allSelected: boolean // kept for external state compatibility
  onChange: (next: { values: string[]; all: boolean }) => void
  hasError?: boolean
}

// Multi-select dropdown without "All" entry; empty or full selection treated as all.
export function AttributesSelect({
  options,
  value,
  allSelected,
  onChange,
  hasError,
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
    <div className="relative text-sm w-full" ref={ref}>
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className={
          `w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] text-left ring-1 flex items-center justify-between ` +
          (hasError
            ? 'ring-red-500/40 bg-red-500/10 focus:outline-none focus:ring-red-500/40'
            : 'ring-white/10 focus:outline-none focus:ring-primary-500')
        }
      >
        <span className="truncate">{label}</span>
        <span className="text-xs opacity-70">{open ? '▲' : '▼'}</span>
      </button>
      {open && (
        <div className="absolute z-20 mt-1 w-full rounded-md border border-white/10 bg-surface-100 p-1 shadow-lg">
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
        </div>
      )}
    </div>
  )
}

// allowed attribute keys (defensive whitelist)
const ALLOWED_ATTR_ORDER: string[] = ['sip', 'dip', 'dport', 'proto']
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
