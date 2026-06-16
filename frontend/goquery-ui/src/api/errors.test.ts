import { describe, it, expect } from 'vitest'
import { extractProblem, apiError, abortError, unknownError, isApiError } from './errors'

describe('extractProblem', () => {
  it('returns the body when it carries a detail field', () => {
    const body = { detail: 'bad request' }
    expect(extractProblem(body)).toBe(body)
  })

  it('returns the body when it carries an errors field', () => {
    const body = { errors: [{ message: 'x' }] }
    expect(extractProblem(body)).toBe(body)
  })

  it('returns undefined for a body without problem fields', () => {
    expect(extractProblem({ something: 1 })).toBeUndefined()
  })

  it('returns undefined for non-object bodies', () => {
    expect(extractProblem(undefined)).toBeUndefined()
    expect(extractProblem('oops')).toBeUndefined()
    expect(extractProblem(null)).toBeUndefined()
  })
})

describe('apiError', () => {
  it('categorizes 5xx as network and 4xx as client', () => {
    expect(apiError(503, undefined).category).toBe('network')
    expect(apiError(400, undefined).category).toBe('client')
  })

  it('carries status, body, and problem through', () => {
    const problem = { detail: 'nope' } as any
    const e = apiError(422, { raw: 1 }, problem)
    expect(e).toMatchObject({ name: 'ApiError', status: 422, body: { raw: 1 }, problem })
  })
})

describe('error guards and factories', () => {
  it('isApiError recognizes constructed errors', () => {
    expect(isApiError(apiError(500, undefined))).toBe(true)
    expect(isApiError(abortError())).toBe(true)
    expect(isApiError(unknownError(new Error('x')))).toBe(true)
    expect(isApiError(new Error('plain'))).toBe(false)
    expect(isApiError(null)).toBe(false)
  })

  it('abortError is a network-category AbortError', () => {
    expect(abortError()).toMatchObject({ name: 'AbortError', category: 'network' })
  })

  it('unknownError preserves the original throwable in body', () => {
    const orig = new Error('boom')
    expect(unknownError(orig)).toMatchObject({ category: 'unknown', body: orig })
  })
})
