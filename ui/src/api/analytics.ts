import { apiFetch } from './client'

const BASE = '/api/v1/analytics'

export interface TokenRecord {
  tenant_id: string
  model: string
  total_input_tokens: number
  total_output_tokens: number
  period_start: string
  period_end: string
}

export interface CostRecord {
  tenant_id: string
  model: string
  total_cost: number
  request_count: number
  period_start: string
  period_end: string
}

export interface SeriesPoint {
  bucket: string
  input_tokens: number
  output_tokens: number
  cost: number
  requests: number
}

interface TokensResult {
  records: TokenRecord[]
  totals: { total_input_tokens: number; total_output_tokens: number }
}

interface CostResult {
  records: CostRecord[]
  totals: { total_cost: number; request_count: number }
}

interface SeriesResult {
  series: SeriesPoint[]
}

export function getAnalyticsTokens(from: string, to: string): Promise<TokensResult> {
  return apiFetch(`${BASE}/tokens?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
}

export function getAnalyticsCost(from: string, to: string): Promise<CostResult> {
  return apiFetch(`${BASE}/cost?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
}

export function getAnalyticsSeries(from: string, to: string): Promise<SeriesResult> {
  return apiFetch(`${BASE}/timeseries?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`)
}
