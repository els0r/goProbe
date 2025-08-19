// Runtime environment accessor. Reads window.__ENV__ injected by Caddy/initContainer or docker-compose
// Falls back to process.env (compile-time) and sensible defaults for local dev.

declare global {
  interface Window {
    __ENV__?: Record<string, string | undefined>
  }
}

const raw = (typeof window !== 'undefined' ? window.__ENV__ : undefined) || {}
// guard access to process.env in browser bundles
const procEnv: Record<string, string | undefined> =
  (typeof process !== 'undefined' && (process as any)?.env) || {}

const str = (v: unknown, d?: string): string => {
  if (v === undefined || v === null) return d ?? ''
  return String(v)
}

const toBool = (v?: string) => ['true', '1', 'yes', 'on'].includes(String(v).trim().toLowerCase())
const toArray = (v?: string) =>
  String(v ?? '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)

// prefer runtime window.__ENV__, then compile-time process.env, then default
const envGet = (key: string, dflt?: string): string => {
  const fromWindow = (raw as any)[key]
  if (fromWindow !== undefined && fromWindow !== null) return String(fromWindow)
  const fromProc = procEnv[key]
  if (fromProc !== undefined && fromProc !== null) return String(fromProc)
  return dflt ?? ''
}

export const env = {
  // API base URL for Global Query backend
  GQ_API_BASE_URL: envGet('GQ_API_BASE_URL', 'http://localhost:8145'),
  HOST_RESOLVER_TYPES: toArray(envGet('HOST_RESOLVER_TYPES', 'string')),
  SSE_ON_LOAD: toBool(envGet('SSE_ON_LOAD', 'true')),
}

export function getApiBaseUrl(): string {
  return env.GQ_API_BASE_URL.replace(/\/$/, '')
}
