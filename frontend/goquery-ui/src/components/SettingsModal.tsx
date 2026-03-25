import React from 'react'
import { env } from '../env'

export interface SettingsModalProps {
  backendUrl: string
  onBackendUrlChange: (url: string) => void
  useStreaming: boolean
  onStreamingChange: (v: boolean) => void
  hostsResolver: string
  onHostsResolverChange: (v: string) => void
  onStreamingReset: () => void
  defaultBackend: string
  showTotalsPercentage: boolean
  onTotalsPercentageChange: (v: boolean) => void
  onClose: () => void
}

export function SettingsModal({
  backendUrl,
  onBackendUrlChange,
  useStreaming,
  onStreamingChange,
  hostsResolver,
  onHostsResolverChange,
  onStreamingReset,
  defaultBackend,
  showTotalsPercentage,
  onTotalsPercentageChange,
  onClose,
}: SettingsModalProps) {
  return (
    <>
      <div
        className="absolute inset-0 z-20 rounded-lg bg-black/50"
        onClick={onClose}
      />
      <div className="absolute left-1/2 top-16 z-30 w-[min(520px,90%)] -translate-x-1/2 rounded-lg border border-white/10 bg-surface-100 p-4 shadow-xl">
        <div className="mb-2 flex items-center justify-between">
          <div className="text-[13px] font-semibold text-gray-200">Settings</div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md bg-surface-200 px-2 py-1 text-data ring-1 ring-white/10 hover:bg-surface-300"
          >
            Close
          </button>
        </div>
        <div className="space-y-3 text-data">
          <div className="flex items-center justify-between">
            <label className="flex items-center gap-2 text-gray-300">
              <input
                type="checkbox"
                checked={useStreaming}
                onChange={(e) => onStreamingChange(e.target.checked)}
              />
              Stream results
            </label>
            <button
              type="button"
              className="text-data-sm text-gray-400 hover:text-gray-200 underline decoration-dotted"
              onClick={onStreamingReset}
              title={`Reset to default (${env.SSE_ON_LOAD ? 'on' : 'off'})`}
            >
              Reset to default ({env.SSE_ON_LOAD ? 'on' : 'off'})
            </button>
          </div>
          <div className="flex items-center">
            <label className="flex items-center gap-2 text-gray-300">
              <input
                type="checkbox"
                checked={showTotalsPercentage}
                onChange={(e) => onTotalsPercentageChange(e.target.checked)}
              />
              Show totals percentage
            </label>
          </div>
          <div className="flex flex-col">
            <label className="mb-1 text-data-sm tracking-wide text-gray-400">Override backend</label>
            <input
              type="text"
              placeholder="Leave empty to use same-origin"
              className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-white/10 focus:outline-none focus:ring-primary-500"
              value={backendUrl}
              onChange={(e) => onBackendUrlChange(e.target.value)}
            />
            <div className="mt-1 text-data-sm text-gray-500">Empty = relative paths via reverse proxy</div>
          </div>
          <div className="flex flex-col">
            <label className="mb-1 text-data-sm tracking-wide text-gray-400">Hosts Resolver</label>
            {env.HOST_RESOLVER_TYPES.length > 0 ? (
              <select
                className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-white/10 focus:outline-none focus:ring-primary-500"
                value={hostsResolver}
                onChange={(e) => onHostsResolverChange(e.target.value)}
              >
                {env.HOST_RESOLVER_TYPES.map((opt: string) => (
                  <option key={opt} value={opt}>
                    {opt}
                  </option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-white/10"
                value="—"
                disabled
              />
            )}
            {env.HOST_RESOLVER_TYPES.length === 0 && (
              <div className="mt-1 text-data-sm text-gray-500">No resolver types configured</div>
            )}
          </div>
        </div>
      </div>
    </>
  )
}
