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
  currency?: string
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

export function withTenant(base: string, tenant?: string): string {
  if (!tenant) return base
  return `${base}${base.includes('?') ? '&' : '?'}tenant=${encodeURIComponent(tenant)}`
}

export function getAnalyticsTokens(from: string, to: string, tenant?: string): Promise<TokensResult> {
  return apiFetch(withTenant(`${BASE}/tokens?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, tenant))
}

export function getAnalyticsCost(from: string, to: string, tenant?: string): Promise<CostResult> {
  return apiFetch(withTenant(`${BASE}/cost?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, tenant))
}

export function getAnalyticsSeries(from: string, to: string, tenant?: string): Promise<SeriesResult> {
  return apiFetch(withTenant(`${BASE}/timeseries?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, tenant))
}
