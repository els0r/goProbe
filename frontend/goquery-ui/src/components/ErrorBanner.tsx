import React from 'react'
import { formatValue } from '../utils/inputSanitize'

export interface ErrorBannerProps {
  error: unknown
}

export function ErrorBanner({ error }: ErrorBannerProps) {
  if (!error) return null
  const [open, setOpen] = React.useState(false)
  let simple = ''
  let problem: any | undefined
  if (typeof error === 'string') simple = error
  else if (error && typeof error === 'object') {
    const e: any = error
    simple = e.message || 'error'
    if (e.problem) problem = e.problem
  }
  return (
    <div className="mb-3 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm">
      <div className="flex items-center justify-between">
        <div className="font-medium text-red-300">{simple}</div>
        {problem && (
          <button
            type="button"
            onClick={() => setOpen((o) => !o)}
            className="text-data-sm rounded px-2 py-0.5 text-red-300 hover:text-white hover:bg-red-500/20 ring-1 ring-red-500/30"
          >
            {open ? 'Hide details' : 'Show details'}
          </button>
        )}
      </div>
      {problem && open && (
        <div className="mt-2 space-y-1 text-red-200/90">
          {problem.detail && <div className="text-data">{problem.detail}</div>}
          {Array.isArray(problem.errors) && problem.errors.length > 0 && (
            <ul className="mt-1 max-h-60 overflow-auto rounded bg-red-500/5 p-2 text-data-sm leading-snug ring-1 ring-red-500/20">
              {problem.errors.map((er: any, i: number) => (
                <li key={i} className="mb-2 last:mb-0">
                  <div>
                    <span className="font-mono text-red-300">{er.location || '(unknown)'}:</span>{' '}
                    {er.message || 'validation error'}
                  </div>
                  {er.value !== undefined &&
                    (typeof er.value === 'object' && er.value !== null ? (
                      <pre className="mt-1 max-h-40 overflow-auto whitespace-pre rounded bg-black/30 p-2 text-data-sm text-red-200/90 ring-1 ring-red-500/20">
                        {JSON.stringify(er.value, null, 2)}
                      </pre>
                    ) : (
                      <div className="opacity-70">value: {formatValue(er.value)}</div>
                    ))}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
