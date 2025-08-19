import React from 'react'
import { IfaceDetailsCard } from './IfaceDetailsCard'

export interface IfaceGroup {
  iface: string
  inB: number
  outB: number
  inP: number
  outP: number
}

export interface IfaceDetailsProps {
  groups: IfaceGroup[]
  loading: boolean
}

export const IfaceDetails: React.FC<IfaceDetailsProps> = ({ groups, loading }) => {
  return (
    <div className="scroll-thin max-h-[45vh] overflow-auto pr-1">
      {loading && <div className="py-8 text-center text-[12px] text-gray-400">Loading…</div>}
      {!loading && groups.length === 0 && (
        <div className="py-8 text-center text-[12px] text-gray-400">No interfaces</div>
      )}
      <div className="space-y-2">
        {groups.map((g, i) => (
          <div key={i}>
            <IfaceDetailsCard
              iface={g.iface}
              inBytes={g.inB}
              inPackets={g.inP}
              outBytes={g.outB}
              outPackets={g.outP}
            />
          </div>
        ))}
      </div>
    </div>
  )
}

export default IfaceDetails
