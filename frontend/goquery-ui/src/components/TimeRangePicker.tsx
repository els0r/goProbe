import React, { useEffect, useRef, useState } from 'react'
import {
  AbsoluteRange,
  QUICK_RANGES,
  browserTimeZone,
  describeTimeRange,
  isRelativeTime,
  loadRecentRanges,
  localInputToAbsolute,
  saveRecentRange,
} from '../utils/timeRange'
import { Chevron } from './Chevron'

export interface TimeRangePickerProps {
  first: string
  last: string
  onApply: (first: string, last: string) => void
  errors?: { first?: string; last?: string }
}

// Grafana-style time range picker. Relative ranges use the backend's native
// syntax ("-3h" → now); absolute picks pass through whatever the backend can
// parse (validated server-side on run).
export function TimeRangePicker({ first, last, onApply, errors }: TimeRangePickerProps) {
  const [open, setOpen] = useState(false)
  const [fromInput, setFromInput] = useState('')
  const [toInput, setToInput] = useState('')
  const [search, setSearch] = useState('')
  const [recents, setRecents] = useState<AbsoluteRange[]>([])
  const ref = useRef<HTMLDivElement | null>(null)
  const hasError = !!(errors?.first || errors?.last)

  useEffect(() => {
    function onDoc(e: MouseEvent) {
      if (!ref.current) return
      if (!ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open])

  function toggleOpen() {
    if (!open) {
      // sync edit buffers with the committed range on open
      setFromInput(first)
      setToInput(last || 'now')
      setSearch('')
      setRecents(loadRecentRanges())
    }
    setOpen((o) => !o)
  }

  function applyQuickRange(value: string) {
    setOpen(false)
    onApply(value, '')
  }

  function applyAbsolute() {
    const from = fromInput.trim()
    const toRaw = toInput.trim()
    const to = toRaw === 'now' ? '' : toRaw
    if (!from) return
    if (!isRelativeTime(from)) {
      saveRecentRange({ first: from, last: to })
    }
    setOpen(false)
    onApply(from, to)
  }

  function applyRecent(r: AbsoluteRange) {
    saveRecentRange(r)
    setOpen(false)
    onApply(r.first, r.last)
  }

  const filteredRanges = QUICK_RANGES.filter((q) =>
    q.label.toLowerCase().includes(search.trim().toLowerCase())
  )
  const [tzName, tzOffset] = browserTimeZone()

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        onClick={toggleOpen}
        title="Change time range"
        className={
          `inline-flex items-center gap-2 rounded-md bg-surface-200 px-3 py-1.5 text-[13px] font-medium ring-1 hover:bg-surface-300 focus:outline-none ` +
          (hasError
            ? 'ring-red-500/40 bg-red-500/10 text-red-200 focus:ring-red-500/40'
            : 'ring-line text-gray-200 focus:ring-primary-500')
        }
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4 opacity-70">
          <path fillRule="evenodd" d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Zm.75-13a.75.75 0 0 0-1.5 0v5c0 .27.144.518.378.65l3.5 2a.75.75 0 1 0 .744-1.3L10.75 9.56V5Z" clipRule="evenodd" />
        </svg>
        <span className="max-w-[340px] truncate">{describeTimeRange(first, last)}</span>
        <Chevron open={open} />
      </button>
      {open && (
        <div className="absolute right-0 z-40 mt-1 w-[580px] rounded-lg border border-line bg-surface-100 shadow-xl text-[13px]">
          <div className="flex">
            {/* left: absolute range + recents */}
            <div className="flex-1 p-4">
              <div className="mb-3 font-semibold text-gray-200">Absolute time range</div>
              <div className="mb-2">
                <label className="mb-1 block text-gray-400">From</label>
                <AbsoluteInput
                  value={fromInput}
                  onChange={setFromInput}
                  onEnter={applyAbsolute}
                  error={errors?.first}
                />
              </div>
              <div className="mb-3">
                <label className="mb-1 block text-gray-400">To</label>
                <AbsoluteInput
                  value={toInput}
                  onChange={setToInput}
                  onEnter={applyAbsolute}
                  error={errors?.last}
                />
              </div>
              <button
                type="button"
                onClick={applyAbsolute}
                disabled={!fromInput.trim()}
                className="btn btn-primary disabled:opacity-50"
              >
                Apply time range
              </button>
              {recents.length > 0 && (
                <div className="mt-4">
                  <div className="mb-1 text-gray-400">Recently used absolute ranges</div>
                  <ul>
                    {recents.map((r, i) => (
                      <li key={r.first + r.last + i}>
                        <button
                          type="button"
                          onClick={() => applyRecent(r)}
                          className="w-full rounded px-2 py-1 text-left text-accent hover:bg-surface-200 hover:text-accent"
                        >
                          {describeTimeRange(r.first, r.last)}
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
            {/* right: quick ranges */}
            <div className="w-56 border-l border-line p-2">
              <input
                type="text"
                placeholder="Search quick ranges"
                className="mb-1 w-full rounded-md bg-surface-200 px-2 py-1 ring-1 ring-line focus:outline-none focus:ring-primary-500"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              <ul className="max-h-72 overflow-auto scroll-thin">
                {filteredRanges.map((q) => {
                  const active = first === q.value && !last
                  return (
                    <li key={q.value}>
                      <button
                        type="button"
                        onClick={() => applyQuickRange(q.value)}
                        className={
                          `w-full rounded px-2 py-1 text-left ` +
                          (active
                            ? 'bg-primary-600 text-on-accent'
                            : 'text-gray-200 hover:bg-surface-200')
                        }
                      >
                        {q.label}
                      </button>
                    </li>
                  )
                })}
                {filteredRanges.length === 0 && (
                  <li className="px-2 py-1 text-gray-400">No matching ranges</li>
                )}
              </ul>
            </div>
          </div>
          <div className="flex items-center justify-between border-t border-line px-4 py-2 text-data-sm text-gray-400">
            <span>
              Browser Time <span className="text-gray-300">{tzName || 'local'}</span>
            </span>
            <span>{tzOffset}</span>
          </div>
        </div>
      )}
    </div>
  )
}

// Free-text time input with a native datetime-local picker behind the calendar
// icon. Accepts anything the backend parses: "-3h", "now", unix timestamps, or
// absolute formats like "2026-06-12 21:40 +0200".
function AbsoluteInput({
  value,
  onChange,
  onEnter,
  error,
}: {
  value: string
  onChange: (v: string) => void
  onEnter: () => void
  error?: string
}) {
  return (
    <div>
      <div className="flex gap-1">
        <input
          type="text"
          className={
            `flex-1 rounded-md bg-surface-200 px-2 py-1 ring-1 focus:outline-none ` +
            (error
              ? 'ring-red-500/40 bg-red-500/10 focus:ring-red-500/40'
              : 'ring-line focus:ring-primary-500')
          }
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              onEnter()
            }
          }}
        />
        <span
          className="relative inline-flex h-[26px] w-8 items-center justify-center rounded-md bg-surface-200 ring-1 ring-line hover:bg-surface-300"
          title="Pick date and time"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4 text-gray-300">
            <path d="M5.25 2A.75.75 0 0 0 4.5 2.75V4H4a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-.5V2.75a.75.75 0 0 0-1.5 0V4H6V2.75A.75.75 0 0 0 5.25 2ZM4 8h12v8H4V8Z" />
          </svg>
          <input
            type="datetime-local"
            className="absolute inset-0 cursor-pointer opacity-0"
            tabIndex={-1}
            onChange={(e) => {
              const abs = localInputToAbsolute(e.target.value)
              if (abs) onChange(abs)
            }}
          />
        </span>
      </div>
      {error && <div className="mt-1 text-red-300">{error}</div>}
    </div>
  )
}
