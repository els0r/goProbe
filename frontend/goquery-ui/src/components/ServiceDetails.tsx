import React from 'react'
import { ServiceDetailsCard } from './ServiceDetailsCard'

export interface ServiceGroup {
  proto: number | null
  dport: number | null
  inB: number
  outB: number
  inP: number
  outP: number
}

export interface ServiceDetailsProps {
  groups: ServiceGroup[]
  loading: boolean
}

export const ServiceDetails: React.FC<ServiceDetailsProps> = ({ groups, loading }) => {
  return (
    <div className="scroll-thin max-h-[45vh] overflow-auto pr-1">
      {loading && <div className="py-8 text-center text-[12px] text-gray-400">Loading…</div>}
      {!loading && groups.length === 0 && (
        <div className="py-8 text-center text-[12px] text-gray-400">No services</div>
      )}
      <div className="space-y-2">
        {groups.map((g, i) => {
          const uni = (g.inB === 0 && g.inP === 0) || (g.outB === 0 && g.outP === 0)
          const backgroundClass = uni
            ? 'bg-red-400/15 ring-1 ring-red-400/20 hover:bg-red-400/25'
            : 'bg-surface-200/60 border-white/10'
          return (
            <ServiceDetailsCard
              key={i}
              proto={g.proto}
              dport={g.dport}
              inBytes={g.inB}
              inPackets={g.inP}
              outBytes={g.outB}
              outPackets={g.outP}
              backgroundClass={backgroundClass}
            />
          )
        })}
      </div>
    </div>
  )
}

export default ServiceDetails
