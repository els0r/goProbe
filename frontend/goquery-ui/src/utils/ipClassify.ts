// classify private IPs (IPv4 RFC1918 A/B/C + loopback/link-local; IPv6 ULA/link-local)
export function isPrivateIP(ip: string): boolean {
  if (!ip) return false
  if (ip.includes(':')) {
    const lower = ip.toLowerCase()
    return (
      lower.startsWith('fc') ||
      lower.startsWith('fd') || // IPv6 ULA
      lower.startsWith('fe80') || // link-local
      lower === '::1' // loopback
    )
  }
  const m = ip.match(/^(\d+)\.(\d+)\./)
  if (!m) return false
  const a = parseInt(m[1], 10)
  const b = parseInt(m[2], 10)
  if (a === 10) return true // 10.0.0.0/8 (Class A private)
  if (a === 172 && b >= 16 && b <= 31) return true // 172.16.0.0/12 (Class B private)
  if (a === 192 && b === 168) return true // 192.168.0.0/16 (Class C private)
  if (a === 127) return true // loopback
  if (a === 169 && b === 254) return true // link-local
  return false
}
