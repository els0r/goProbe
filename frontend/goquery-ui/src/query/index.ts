// query/ — the Query concept (CONTEXT.md), owned end to end: its shape
// (params), URL form (serialize), and execution (runner, detailRunner,
// autoRun). Barrel re-export of the public surface, mirroring flows/. See
// ADR-0003.
export * from './params'
export * from './serialize'
export * from './runner'
export * from './detailRunner'
export * from './autoRun'
export * from './useQueryRunner'
export * from './useDetailRunner'
