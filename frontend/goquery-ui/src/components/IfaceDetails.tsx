import React from 'react'
import { IfaceDetailsCard } from './IfaceDetailsCard'
import { DetailList } from './DetailList'
import { IfaceGroup } from '../utils/aggregation'

export type { IfaceGroup }

export interface IfaceDetailsProps {
  groups: IfaceGroup[]
  loading: boolean
}

export const IfaceDetails: React.FC<IfaceDetailsProps> = ({ groups, loading }) => (
  <DetailList
    items={groups}
    loading={loading}
    emptyText="No interfaces"
    renderItem={(g: IfaceGroup) => (
      <IfaceDetailsCard
        iface={g.iface}
        inBytes={g.inB}
        inPackets={g.inP}
        outBytes={g.outB}
        outPackets={g.outP}
      />
    )}
  />
)

export default IfaceDetails
