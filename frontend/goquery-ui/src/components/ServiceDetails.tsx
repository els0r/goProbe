import React from 'react'
import { ServiceDetailsCard } from './ServiceDetailsCard'
import { DetailList } from './DetailList'
import { ServiceGroup } from '../utils/aggregation'

export type { ServiceGroup }

export interface ServiceDetailsProps {
  groups: ServiceGroup[]
  loading: boolean
}

export const ServiceDetails: React.FC<ServiceDetailsProps> = ({ groups, loading }) => (
  <DetailList
    items={groups}
    loading={loading}
    emptyText="No services"
    renderItem={(g: ServiceGroup) => {
      const uni = (g.inB === 0 && g.inP === 0) || (g.outB === 0 && g.outP === 0)
      const backgroundClass = uni
        ? 'bg-red-400/15 ring-1 ring-red-400/20 hover:bg-red-400/25'
        : 'bg-surface-200/60 border-white/10'
      return (
        <ServiceDetailsCard
          proto={g.proto}
          dport={g.dport}
          inBytes={g.inB}
          inPackets={g.inP}
          outBytes={g.outB}
          outPackets={g.outP}
          backgroundClass={backgroundClass}
        />
      )
    }}
  />
)

export default ServiceDetails
