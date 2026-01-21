import React from 'react'
import { DetailsCard } from './DetailsCard'

export interface IfaceDetailsCardProps {
  iface: string
  inBytes: number
  inPackets: number
  outBytes: number
  outPackets: number
  backgroundClass?: string
  className?: string
}

export const IfaceDetailsCard: React.FC<IfaceDetailsCardProps> = ({
  iface,
  inBytes,
  inPackets,
  outBytes,
  outPackets,
  backgroundClass,
  className,
}) => {
  const totalBytes = inBytes + outBytes
  const totalPackets = inPackets + outPackets
  return (
    <DetailsCard
      heading={iface}
      totalBytes={totalBytes}
      totalPackets={totalPackets}
      inBytes={inBytes}
      inPackets={inPackets}
      outBytes={outBytes}
      outPackets={outPackets}
      backgroundClass={backgroundClass}
      className={className}
    />
  )
}

export default IfaceDetailsCard
