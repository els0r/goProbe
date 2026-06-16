// domain.ts - curated type aliases over the generated OpenAPI schema.
//
// The Flow domain (FlowRecord, totals, grouping) lives in src/flows/; the Query
// shape (QueryParamsUI) lives in src/query/params.ts; the ApiError type and its
// guards live in src/api/errors.ts. What remains here is purely transport: thin
// aliases over the generated schema (see ADR-0003).
// GENERATED SCHEMA TYPES: import from './generated'

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore: generated file may not exist until `make types` runs
import type { components } from './generated'

// type aliases for clarity
export type ResultSchema = components['schemas']['Result']
export type RowSchema = components['schemas']['Row']
export type CountersSchema = components['schemas']['Counters']
export type AttributesSchema = components['schemas']['Attributes']
export type LabelsSchema = components['schemas']['Labels']
export type ErrorModelSchema = components['schemas']['ErrorModel']
export type SummarySchema = components['schemas']['Summary']

