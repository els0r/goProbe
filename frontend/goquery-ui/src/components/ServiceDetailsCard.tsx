import React from 'react'
import { DetailsCard } from './DetailsCard'
import { renderProto } from '../utils/proto'

export interface ServiceDetailsCardProps {
  proto: number | null
  dport: number | null
  inBytes: number
  inPackets: number
  outBytes: number
  outPackets: number
  backgroundClass?: string
  className?: string
}

// wraps DetailsCard and centralizes service heading rendering (proto[/dport])
export const ServiceDetailsCard: React.FC<ServiceDetailsCardProps> = ({
  proto,
  dport,
  inBytes,
  inPackets,
  outBytes,
  outPackets,
  backgroundClass,
  className,
}) => {
  const heading = `${renderProto(proto)}${dport !== null && dport !== undefined ? '/' + dport : ''}`
  const totalBytes = inBytes + outBytes
  const totalPackets = inPackets + outPackets
  return (
    <DetailsCard
      heading={heading}
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

export default ServiceDetailsCard

