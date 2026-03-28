import React from 'react'
import { FlowRecord, SummarySchema } from '../api/domain'
import { ProgressBar } from '../components/ProgressBar'
import { humanBytes, humanPackets } from '../utils/format'
import { renderProto } from '../utils/proto'
import { TemporalRowDetails } from './TemporalRowDetails'

export interface TableViewProps {
  rows: FlowRecord[]
  loading: boolean
  streaming?: boolean
  attributes?: string[] | null
  onRowClick?: (row: FlowRecord) => void
  selectedRow?: FlowRecord | null
  totalsBytes?: number
  totalsPackets?: number
  showTotalsPercentage?: boolean
  temporalDetail?: {
    meta: { host: string; host_id: string; iface: string; sip: string; dip: string; dport?: number | null; proto?: number | null }
    attrsShown: string[]
    rows: FlowRecord[]
    loading: boolean
    error?: unknown
    summary?: SummarySchema
  } | null
  copyMeta?: {
    first?: string
    last?: string
    interfacesCount?: number
    hostsTotal?: number
    hostsOk?: number
    hostsErrors?: number
    sortBy?: 'bytes' | 'packets'
    hitsTotal?: number
    durationNs?: number
    condition?: string
    br?: number
    bs?: number
    pr?: number
    ps?: number
  }
}

export function TableView({
  rows,
  loading,
  streaming,
  attributes,
  onRowClick,
  selectedRow,
  totalsBytes,
  totalsPackets,
  showTotalsPercentage = false,
  temporalDetail,
  copyMeta,
}: TableViewProps) {
  const showAll = !attributes || attributes.length === 0
  const show = (attr: string) => showAll || attributes.includes(attr)
  const anyHost = rows.some((r) => !!r.host)
  const anyIface = rows.some((r) => !!r.iface)
  const isEmpty = rows.length === 0
  // count visible columns for colSpan
  const colCount =
    (anyHost ? 1 : 0) +
    (anyIface ? 1 : 0) +
    (show('sip') ? 1 : 0) +
    (show('dip') ? 1 : 0) +
    (show('dport') ? 1 : 0) +
    (show('proto') ? 1 : 0) +
    6 // bytes in/out/total + packets in/out/total

  return (
    <table className="min-w-full border-collapse text-left text-sm">
      <thead className="table-header text-xs uppercase tracking-wide text-gray-400">
        <tr>
          {anyHost && (
            <th key="host" className="px-2 py-2 font-medium">
              host
            </th>
          )}
          {anyIface && (
            <th key="interface" className="px-2 py-2 font-medium">
              interface
            </th>
          )}
          {show('sip') && (
            <th key="sip" className="px-2 py-2 font-medium">
              sip
            </th>
          )}
          {show('dip') && (
            <th key="dip" className="px-2 py-2 font-medium">
              dip
            </th>
          )}
          {show('dport') && (
            <th key="dport" className="px-2 py-2 font-medium text-right">
              dport
            </th>
          )}
          {show('proto') && (
            <th key="proto" className="px-2 py-2 font-medium">
              proto
            </th>
          )}
          <th key="bytes_in" className="px-2 py-2 font-medium text-right">
            bytes in
          </th>
          <th key="bytes_out" className="px-2 py-2 font-medium text-right">
            bytes out
          </th>
          <th key="bytes_total" className="px-2 py-2 font-medium text-right">
            bytes total
          </th>
          <th key="packets_in" className="px-2 py-2 font-medium text-right">
            packets in
          </th>
          <th key="packets_out" className="px-2 py-2 font-medium text-right">
            packets out
          </th>
          <th key="packets_total" className="px-2 py-2 font-medium text-right">
            packets total
          </th>
        </tr>
      </thead>
      <tbody className="divide-y divide-white/5">
        {isEmpty && (
          <tr>
            <td colSpan={12} className="px-2 py-3 text-center text-data text-gray-400">
              {loading ? 'Loading…' : streaming ? 'Waiting for partial results…' : 'No results'}
            </td>
          </tr>
        )}
        {rows.map((r, i) => {
          const uni = !r.bidirectional
          const isSelected =
            selectedRow != null &&
            selectedRow.sip === r.sip &&
            selectedRow.dip === r.dip &&
            selectedRow.dport === r.dport &&
            selectedRow.proto === r.proto &&
            selectedRow.host_id === r.host_id &&
            selectedRow.iface === r.iface
          const rowClass = isSelected
            ? 'ring-1 ring-inset ring-primary-400/60 bg-primary-400/10'
            : uni
              ? 'bg-red-400/15 hover:bg-red-400/25'
              : 'hover:bg-surface-100/60'
          const prev = i > 0 ? rows[i - 1] : undefined
          const tupleChanged = i === 0 || r.host !== prev?.host || r.iface !== prev?.iface
          return (
            <React.Fragment key={i}>
              <tr
                className={`${rowClass} ${onRowClick ? 'cursor-pointer' : ''}`}
                data-unidirectional={uni || undefined}
                aria-selected={isSelected || undefined}
                tabIndex={onRowClick ? 0 : undefined}
                onClick={onRowClick ? () => onRowClick(r) : undefined}
                onKeyDown={
                  onRowClick
                    ? (e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        onRowClick(r)
                      }
                    }
                    : undefined
                }
              >
                {anyHost && (
                  <td key="host" className="px-2 py-2 text-data" title={r.host_id || ''}>
                    {tupleChanged ? r.host || '' : ''}
                  </td>
                )}
                {anyIface && (
                  <td key="interface" className="px-2 py-2 text-data">
                    {tupleChanged ? r.iface || '' : ''}
                  </td>
                )}
                {show('sip') && (
                  <td key="sip" className="px-2 py-2 text-data">
                    {r.sip}
                  </td>
                )}
                {show('dip') && (
                  <td key="dip" className="px-2 py-2 text-data">
                    {r.dip}
                  </td>
                )}
                {show('dport') && (
                  <td key="dport" className="px-2 py-2 tabular-nums text-right text-data">
                    {r.dport ?? ''}
                  </td>
                )}
                {show('proto') && (
                  <td
                    key="proto"
                    className="px-2 py-2 text-data"
                    title={r.proto !== undefined && r.proto !== null ? String(r.proto) : ''}
                  >
                    {renderProto(r.proto as any)}
                  </td>
                )}
                <td
                  key="bytes_in"
                  className="px-2 py-2 tabular-nums text-right text-data text-gray-300"
                  title={String(r.bytes_in)}
                >
                  {humanBytes(r.bytes_in)}
                </td>
                <td
                  key="bytes_out"
                  className="px-2 py-2 tabular-nums text-right text-data text-gray-300"
                  title={String(r.bytes_out)}
                >
                  {humanBytes(r.bytes_out)}
                </td>
                <td
                  key="bytes_total"
                  className="px-2 py-2 tabular-nums text-right text-data text-primary-300 font-medium"
                  title={String(r.bytes_in + r.bytes_out)}
                >
                  <div className="flex w-full flex-col items-end">
                    <div>{humanBytes(r.bytes_in + r.bytes_out)}</div>
                    {showTotalsPercentage && (() => {
                      const total = Math.max(0, Number(totalsBytes) || 0)
                      const share = total > 0 ? (r.bytes_in + r.bytes_out) / total : 0
                      const pctLinear = Math.max(0, Math.min(100, share * 100))
                      return (
                        <ProgressBar
                          percent={pctLinear}
                          title={total > 0 ? `${pctLinear.toFixed(1)} %` : '—'}
                        />
                      )
                    })()}
                  </div>
                </td>
                <td
                  key="packets_in"
                  className="px-2 py-2 tabular-nums text-right text-data text-gray-300"
                  title={String(r.packets_in)}
                >
                  {humanPackets(r.packets_in)}
                </td>
                <td
                  key="packets_out"
                  className="px-2 py-2 tabular-nums text-right text-data text-gray-300"
                  title={String(r.packets_out)}
                >
                  {humanPackets(r.packets_out)}
                </td>
                <td
                  key="packets_total"
                  className="px-2 py-2 tabular-nums text-right text-data text-primary-300 font-medium"
                  title={String(r.packets_in + r.packets_out)}
                >
                  <div className="flex w-full flex-col items-end">
                    <div>{humanPackets(r.packets_in + r.packets_out)}</div>
                    {showTotalsPercentage && (() => {
                      const total = Math.max(0, Number(totalsPackets) || 0)
                      const share = total > 0 ? (r.packets_in + r.packets_out) / total : 0
                      const pctLinear = Math.max(0, Math.min(100, share * 100))
                      return (
                        <ProgressBar
                          percent={pctLinear}
                          title={total > 0 ? `${pctLinear.toFixed(1)} %` : '—'}
                        />
                      )
                    })()}
                  </div>
                </td>
              </tr>
              {isSelected && temporalDetail && (
                <TemporalRowDetails
                  rows={temporalDetail.rows}
                  loading={temporalDetail.loading}
                  error={temporalDetail.error}
                  summary={temporalDetail.summary}
                  colSpan={colCount}
                  queryFirst={copyMeta?.first}
                  queryLast={copyMeta?.last}
                  meta={temporalDetail.meta}
                  attrsShown={temporalDetail.attrsShown}
                  originalCondition={copyMeta?.condition}
                />
              )}
            </React.Fragment>
          )
        })}
      </tbody>
    </table>
  )
}
