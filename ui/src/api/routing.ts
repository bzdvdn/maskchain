import { apiFetch } from './client'

export interface ProviderDto {
  name: string
  api_type: string
  base_url: string
  health_endpoint?: string
  timeout?: string
  priority?: number
  api_keys?: string[]
  auth_scheme?: string
  auth_header?: string
  auth_prefix?: string
  additional_headers?: Record<string, string>
  proxy_url?: string
  aws_region?: string
  aws_access_key_id?: string
  aws_secret_access_key?: string
  source?: string
  status?: string
  latency_ms?: number
  last_check?: number
  // models attached on save (global default routes)
  models?: string[]
}

export interface ModelAggregate {
  model: string
  input_price_per_1k: number
  output_price_per_1k: number
  currency: string
  default_providers: string[]
  override_count: number
  source?: string
}

export interface RouteDto {
  tenant: string
  model: string
  providers: string[]
  source?: string
}

export interface CostRateDto {
  model: string
  input_price_per_1k: number
  output_price_per_1k: number
  currency: string
  source?: string
}

const ROUTING = '/api/v1/routing'
const COST = '/api/v1/analytics/cost-rates'

// GLOBAL_TENANT is the reserved tenant value for a model's default provider
// chain (applies to every tenant without an explicit override).
export const GLOBAL_TENANT = '*'

/**
 * isMaskedKey reports whether a provider secret value is the masked display
 * form returned by the admin API (contains the "***" marker) rather than a
 * real key. Used by the Routing page to keep existing secrets untouched when a
 * provider is re-saved without changing its keys.
 */
export function isMaskedKey(v: string | undefined): boolean {
  return !!v && v.includes('***')
}

function unwrap<T>(d: unknown): T {
  if (d && typeof d === 'object' && 'data' in (d as Record<string, unknown>)) {
    return (d as { data: T }).data
  }
  return d as T
}

export function listProviders(): Promise<ProviderDto[]> {
  return apiFetch(`${ROUTING}/providers`).then((d) => unwrap<ProviderDto[]>(d))
}

export function upsertProvider(p: ProviderDto): Promise<ProviderDto> {
  return apiFetch(`${ROUTING}/providers`, { method: 'PUT', body: p })
}

export function deleteProvider(name: string): Promise<void> {
  return apiFetch(`${ROUTING}/providers/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

// deleteProviderModel removes a single model from a provider's catalog (drops
// the provider from the model's global route).
export function deleteProviderModel(name: string, model: string): Promise<void> {
  return apiFetch(`${ROUTING}/providers/${encodeURIComponent(name)}/models/${encodeURIComponent(model)}`, {
    method: 'DELETE',
  })
}

export function listRoutes(): Promise<RouteDto[]> {
  return apiFetch(`${ROUTING}/routes`).then((d) => unwrap<RouteDto[]>(d))
}

export function upsertRoute(r: RouteDto): Promise<RouteDto> {
  return apiFetch(`${ROUTING}/routes`, { method: 'PUT', body: r })
}

export function deleteRoute(r: RouteDto): Promise<void> {
  return apiFetch(`${ROUTING}/routes`, {
    method: 'DELETE',
    body: { tenant: r.tenant, model: r.model, providers: r.providers },
  })
}

// listProviderModels asks the provider's own models API for its model ids.
export function listProviderModels(name: string): Promise<string[]> {
  return apiFetch(`${ROUTING}/providers/${encodeURIComponent(name)}/models`).then((d) => unwrap<string[]>(d))
}

// listModels returns the model aggregate (cost + default providers + override count).
export function listModels(): Promise<ModelAggregate[]> {
  return apiFetch(`${ROUTING}/models`).then((d) => unwrap<ModelAggregate[]>(d))
}

export function listCostRates(): Promise<CostRateDto[]> {
  return apiFetch(COST).then((d) => unwrap<CostRateDto[]>(d))
}

export function upsertCostRate(c: CostRateDto): Promise<CostRateDto> {
  return apiFetch(COST, { method: 'PUT', body: c })
}

export function deleteCostRate(model: string): Promise<void> {
  return apiFetch(`${COST}/${encodeURIComponent(model)}`, { method: 'DELETE' })
}