import React, { useMemo } from 'react'
import { FlowRecord, inOutScaleMax, runSharePct } from '../flows'
import { ProgressBar } from '../components/ProgressBar'
import { InOutBar } from '../components/InOutBar'
import { humanBytes, humanPackets } from '../utils/format'
import { renderProto } from '../utils/proto'
import { TemporalRowDetails } from './TemporalRowDetails'
import { TemporalPanelSnapshot, DrillSnapshot, DrillBucket } from '../query/detailRunner'

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
  visualInOutBars?: boolean
  showDirectionValues?: boolean
  temporalDetail?: TemporalPanelSnapshot | null
  drill?: DrillSnapshot | null
  onDrill?: (bucket: DrillBucket, attrs: { values: string[]; all: boolean }) => void
  onCloseDrill?: () => void
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
  visualInOutBars = false,
  showDirectionValues = false,
  temporalDetail,
  drill,
  onDrill,
  onCloseDrill,
  copyMeta,
}: TableViewProps) {
  const showAll = !attributes || attributes.length === 0
  const show = (attr: string) => showAll || attributes.includes(attr)
  const anyHost = rows.some((r) => !!r.host)
  const anyIface = rows.some((r) => !!r.iface)
  const isEmpty = rows.length === 0
  // shared global max per metric — one scale every diverging bar normalizes
  // against, recomputed live as streaming rows arrive (Q2/Q10)
  const bytesScaleMax = useMemo(
    () => inOutScaleMax(rows, 'bytes_in', 'bytes_out'),
    [rows],
  )
  const packetsScaleMax = useMemo(
    () => inOutScaleMax(rows, 'packets_in', 'packets_out'),
    [rows],
  )
  // visual mode collapses each in/out pair into one gauge cell; the total
  // keeps its own right-aligned column so bars and numbers each stack cleanly
  const bytesCols = visualInOutBars ? 2 : 3
  const packetsCols = visualInOutBars ? 2 : 3
  // count visible columns for colSpan
  const colCount =
    (anyHost ? 1 : 0) +
    (anyIface ? 1 : 0) +
    (show('sip') ? 1 : 0) +
    (show('dip') ? 1 : 0) +
    (show('dport') ? 1 : 0) +
    (show('proto') ? 1 : 0) +
    bytesCols +
    packetsCols

  return (
    <table
      className={
        'min-w-full border-collapse text-left text-sm' +
        // visual mode: tighten body rows so the diverging bars stack into a
        // continuous vertical flow; scoped to direct cells so it never leaks
        // into the nested temporal-detail / drill-down tables
        (visualInOutBars ? ' [&>tbody>tr>td]:py-1 [&>tbody>tr>td]:leading-tight' : '')
      }
    >
      <thead className="table-header text-xs uppercase tracking-wide text-gray-400">
        <tr>
          {anyHost && (
            <th key="host" className="px-2 py-2 font-medium align-bottom">
              host
            </th>
          )}
          {anyIface && (
            <th key="interface" className="px-2 py-2 font-medium align-bottom">
              interface
            </th>
          )}
          {show('sip') && (
            <th key="sip" className="px-2 py-2 font-medium align-bottom">
              sip
            </th>
          )}
          {show('dip') && (
            <th key="dip" className="px-2 py-2 font-medium align-bottom">
              dip
            </th>
          )}
          {show('dport') && (
            <th key="dport" className="px-2 py-2 font-medium text-right align-bottom">
              dport
            </th>
          )}
          {show('proto') && (
            <th key="proto" className="px-2 py-2 font-medium align-bottom">
              proto
            </th>
          )}
          {visualInOutBars ? (
            <>
              <th key="bytes" className="px-2 py-2 font-medium">
                <div className="mx-auto flex w-40 flex-col leading-tight text-gray-400">
                  <span className="text-center text-gray-300">bytes</span>
                  <span className="flex w-full items-center">
                    <span className="w-1/2 pr-1 text-right">in ◄</span>
                    <span className="w-1/2 pl-1 text-left">► out</span>
                  </span>
                </div>
              </th>
              <th key="bytes_total" className="px-2 py-2 font-medium text-right align-bottom">
                total
              </th>
            </>
          ) : (
            <>
              <th key="bytes_in" className="px-2 py-2 font-medium text-right">
                bytes in
              </th>
              <th key="bytes_out" className="px-2 py-2 font-medium text-right">
                bytes out
              </th>
              <th key="bytes_total" className="px-2 py-2 font-medium text-right">
                bytes total
              </th>
            </>
          )}
          {visualInOutBars ? (
            <>
              <th key="packets" className="px-2 py-2 font-medium">
                <div className="mx-auto flex w-40 flex-col leading-tight text-gray-400">
                  <span className="text-center text-gray-300">packets</span>
                  <span className="flex w-full items-center">
                    <span className="w-1/2 pr-1 text-right">in ◄</span>
                    <span className="w-1/2 pl-1 text-left">► out</span>
                  </span>
                </div>
              </th>
              <th key="packets_total" className="px-2 py-2 font-medium text-right align-bottom">
                total
              </th>
            </>
          ) : (
            <>
              <th key="packets_in" className="px-2 py-2 font-medium text-right">
                packets in
              </th>
              <th key="packets_out" className="px-2 py-2 font-medium text-right">
                packets out
              </th>
              <th key="packets_total" className="px-2 py-2 font-medium text-right">
                packets total
              </th>
            </>
          )}
        </tr>
      </thead>
      <tbody className="divide-y divide-line-soft">
        {isEmpty && (
          <tr>
            <td colSpan={colCount} className="px-2 py-3 text-center text-data text-gray-400">
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
          // in visual mode the red unidirectional signal moves onto the bar,
          // so the whole-row tint is dropped to keep the bars reading cleanly
          const baseRowClass = isSelected
            ? 'ring-1 ring-inset ring-primary-400/60 bg-primary-400/10'
            : uni && !visualInOutBars
              ? 'bg-red-400/15 hover:bg-red-400/25'
              : 'hover:bg-surface-100/60'
          // While a committed drill is open, recede every other host row so the
          // elevated drill-down card reads as the figure. Applied to the cells —
          // opacity on <tr> is unreliable. Hover restores full opacity so the
          // table stays an inviting re-targeting surface. (drill != null is
          // already the committed-and-not-dragging signal — see detailRunner.)
          const dimClass =
            drill != null && !isSelected
              ? ' [&>td]:opacity-35 [&>td]:transition-opacity [&>td]:duration-150 hover:[&>td]:opacity-100'
              : ''
          const rowClass = baseRowClass + dimClass
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
                {visualInOutBars ? (
                  <>
                    <td key="bytes" className="px-2 py-2">
                      <InOutBar
                        inValue={r.bytes_in}
                        outValue={r.bytes_out}
                        scaleMax={bytesScaleMax}
                        unidirectional={uni}
                        showValues={showDirectionValues}
                        format={humanBytes}
                      />
                    </td>
                    <td
                      key="bytes_total"
                      className="px-2 py-2 tabular-nums text-right text-data text-accent font-medium"
                      title={String(r.bytes_total)}
                    >
                      <div className="flex w-full flex-col items-end">
                        <div>{humanBytes(r.bytes_total)}</div>
                        {showTotalsPercentage && (() => {
                          const total = Math.max(0, Number(totalsBytes) || 0)
                          const pctLinear = runSharePct(r.bytes_total, total)
                          return (
                            <ProgressBar
                              percent={pctLinear}
                              title={total > 0 ? `${pctLinear.toFixed(1)} %` : '—'}
                            />
                          )
                        })()}
                      </div>
                    </td>
                  </>
                ) : (
                  <>
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
                      className="px-2 py-2 tabular-nums text-right text-data text-accent font-medium"
                      title={String(r.bytes_total)}
                    >
                      <div className="flex w-full flex-col items-end">
                        <div>{humanBytes(r.bytes_total)}</div>
                        {showTotalsPercentage && (() => {
                          const total = Math.max(0, Number(totalsBytes) || 0)
                          const pctLinear = runSharePct(r.bytes_total, total)
                          return (
                            <ProgressBar
                              percent={pctLinear}
                              title={total > 0 ? `${pctLinear.toFixed(1)} %` : '—'}
                            />
                          )
                        })()}
                      </div>
                    </td>
                  </>
                )}
                {visualInOutBars ? (
                  <>
                    <td key="packets" className="px-2 py-2">
                      <InOutBar
                        inValue={r.packets_in}
                        outValue={r.packets_out}
                        scaleMax={packetsScaleMax}
                        unidirectional={uni}
                        showValues={showDirectionValues}
                        format={humanPackets}
                      />
                    </td>
                    <td
                      key="packets_total"
                      className="px-2 py-2 tabular-nums text-right text-data text-accent font-medium"
                      title={String(r.packets_total)}
                    >
                      <div className="flex w-full flex-col items-end">
                        <div>{humanPackets(r.packets_total)}</div>
                        {showTotalsPercentage && (() => {
                          const total = Math.max(0, Number(totalsPackets) || 0)
                          const pctLinear = runSharePct(r.packets_total, total)
                          return (
                            <ProgressBar
                              percent={pctLinear}
                              title={total > 0 ? `${pctLinear.toFixed(1)} %` : '—'}
                            />
                          )
                        })()}
                      </div>
                    </td>
                  </>
                ) : (
                  <>
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
                      className="px-2 py-2 tabular-nums text-right text-data text-accent font-medium"
                      title={String(r.packets_total)}
                    >
                      <div className="flex w-full flex-col items-end">
                        <div>{humanPackets(r.packets_total)}</div>
                        {showTotalsPercentage && (() => {
                          const total = Math.max(0, Number(totalsPackets) || 0)
                          const pctLinear = runSharePct(r.packets_total, total)
                          return (
                            <ProgressBar
                              percent={pctLinear}
                              title={total > 0 ? `${pctLinear.toFixed(1)} %` : '—'}
                            />
                          )
                        })()}
                      </div>
                    </td>
                  </>
                )}
              </tr>
              {isSelected && temporalDetail && (
                <TemporalRowDetails
                  rows={temporalDetail.rows}
                  loading={temporalDetail.phase === 'loading'}
                  error={temporalDetail.error ?? undefined}
                  summary={temporalDetail.summary}
                  colSpan={colCount}
                  queryFirst={copyMeta?.first}
                  queryLast={copyMeta?.last}
                  meta={temporalDetail.meta}
                  attrsShown={temporalDetail.attrsShown}
                  visualInOutBars={visualInOutBars}
                  showDirectionValues={showDirectionValues}
                  drill={drill}
                  onDrill={onDrill}
                  onCloseDrill={onCloseDrill}
                />
              )}
            </React.Fragment>
          )
        })}
      </tbody>
    </table>
  )
}
