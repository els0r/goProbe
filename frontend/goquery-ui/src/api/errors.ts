// Error construction contract for the API layer: the ApiError type, its guards,
// and how a Response body or a thrown value becomes one. ApiError is what
// crosses the runner seam (ADR-0001/0002); this is a transport concern, hence
// its home in api/ rather than the Flow domain (ADR-0003).
import type { ErrorModelSchema } from './domain'

export type ApiErrorCategory = 'network' | 'client' | 'parse' | 'unknown'

export interface ApiError extends Error {
  status?: number
  category: ApiErrorCategory
  body?: unknown
  problem?: ErrorModelSchema
}

// single source of truth for the ApiError type guard
export function isApiError(e: unknown): e is ApiError {
  return !!e && typeof e === 'object' && 'category' in e
}

// Coerce any thrown value into an ApiError without losing an already-typed one.
export function toApiError(e: unknown): ApiError {
  if (isApiError(e)) return e
  return {
    name: 'UnknownError',
    message: e instanceof Error ? e.message : 'unknown error',
    category: 'unknown',
    body: e,
  }
}

// Recover an RFC-7807-ish problem document from a parsed response body, if one
// is present. Returns undefined when the body carries no problem fields.
export function extractProblem(body: unknown): ErrorModelSchema | undefined {
  if (
    body &&
    typeof body === 'object' &&
    ('detail' in (body as any) || 'errors' in (body as any))
  ) {
    return body as ErrorModelSchema
  }
  return undefined
}

export function apiError(status: number, body: unknown, problem?: ErrorModelSchema): ApiError {
  return {
    name: 'ApiError',
    message: `api request failed: status=${status}`,
    status,
    category: status >= 500 ? 'network' : 'client',
    body,
    problem,
  }
}

export function abortError(): ApiError {
  return {
    name: 'AbortError',
    message: 'request aborted',
    category: 'network',
  }
}

export function unknownError(err: unknown): ApiError {
  return {
    name: 'UnknownError',
    message: 'unknown error',
    category: 'unknown',
    body: err,
  }
}

export async function safeJson(res: Response): Promise<unknown> {
  try {
    return await res.json()
  } catch {
    return undefined
  }
}
