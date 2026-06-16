import React from 'react'
import { env } from '../env'
import { ThemePreference } from '../theme'

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
  visualInOutBars: boolean
  onVisualInOutBarsChange: (v: boolean) => void
  showDirectionValues: boolean
  onShowDirectionValuesChange: (v: boolean) => void
  themePreference: ThemePreference
  onThemePreferenceChange: (p: ThemePreference) => void
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
  visualInOutBars,
  onVisualInOutBarsChange,
  showDirectionValues,
  onShowDirectionValuesChange,
  themePreference,
  onThemePreferenceChange,
  onClose,
}: SettingsModalProps) {
  const themeOptions: Array<{ value: ThemePreference; label: string }> = [
    { value: 'system', label: 'System' },
    { value: 'light', label: 'Light' },
    { value: 'dark', label: 'Dark' },
  ]
  return (
    <>
      <div
        className="absolute inset-0 z-20 rounded-lg bg-scrim"
        onClick={onClose}
      />
      <div className="absolute left-1/2 top-16 z-30 w-[min(520px,90%)] -translate-x-1/2 rounded-lg border border-line bg-surface-100 p-4 shadow-xl">
        <div className="mb-2 flex items-center justify-between">
          <div className="text-[13px] font-semibold text-gray-200">Settings</div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md bg-surface-200 px-2 py-1 text-data ring-1 ring-line hover:bg-surface-300"
          >
            Close
          </button>
        </div>
        <div className="space-y-3 text-data">
          <div className="space-y-2">
            <div className="text-data-sm font-semibold uppercase tracking-wide text-gray-400">
              Appearance
            </div>
            <div className="inline-flex gap-1 rounded-md bg-surface-200 p-0.5 ring-1 ring-line">
              {themeOptions.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => onThemePreferenceChange(opt.value)}
                  className={`rounded-md px-3 py-1 text-data transition-colors ${
                    themePreference === opt.value
                      ? 'bg-primary-500 text-on-accent'
                      : 'text-gray-300 hover:bg-surface-300'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
          <div className="flex items-center justify-between border-t border-line pt-3">
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
          <div className="space-y-2 border-t border-line pt-3">
            <div className="text-data-sm font-semibold uppercase tracking-wide text-gray-400">
              Table
            </div>
            <div className="flex items-center">
              <label className="flex items-center gap-2 text-gray-300">
                <input
                  type="checkbox"
                  checked={visualInOutBars}
                  onChange={(e) => onVisualInOutBarsChange(e.target.checked)}
                />
                Visual in/out bars
              </label>
            </div>
            <div className="ml-6 flex items-center">
              <label
                className={`flex items-center gap-2 ${visualInOutBars ? 'text-gray-300' : 'text-gray-500'}`}
              >
                <input
                  type="checkbox"
                  checked={showDirectionValues}
                  disabled={!visualInOutBars}
                  onChange={(e) => onShowDirectionValuesChange(e.target.checked)}
                />
                Show direction values
              </label>
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
          </div>
          <div className="flex flex-col">
            <label className="mb-1 text-data-sm tracking-wide text-gray-400">Override backend</label>
            <input
              type="text"
              placeholder="Leave empty to use same-origin"
              className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-line focus:outline-none focus:ring-primary-500"
              value={backendUrl}
              onChange={(e) => onBackendUrlChange(e.target.value)}
            />
            <div className="mt-1 text-data-sm text-gray-500">Empty = relative paths via reverse proxy</div>
          </div>
          <div className="flex flex-col">
            <label className="mb-1 text-data-sm tracking-wide text-gray-400">Hosts Resolver</label>
            {env.HOST_RESOLVER_TYPES.length > 0 ? (
              <select
                className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-line focus:outline-none focus:ring-primary-500"
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
                className="w-full rounded-md bg-surface-200 px-2 py-1 text-[13px] ring-1 ring-line"
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
