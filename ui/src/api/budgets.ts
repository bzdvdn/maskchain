import { apiFetch } from './client'

const BASE = '/api/v1/budgets'

export interface BudgetDto {
  id: string
  tenant_id: string
  virtual_key_id: string
  model: string
  scope: string
  type: string
  custom_days: number
  soft_limit?: number
  hard_limit?: number
  currency: string
  notify_at: number[]
  enabled: boolean
  spent: number
  created_at: string
  updated_at: string
}

export interface CreateBudgetRequest {
  tenant_id: string
  virtual_key_id?: string
  model?: string
  scope: string
  type: string
  custom_days?: number
  soft_limit?: number
  hard_limit?: number
  currency?: string
  notify_at?: number[]
}

export interface UpdateBudgetRequest {
  virtual_key_id?: string
  model?: string
  scope?: string
  type?: string
  custom_days?: number
  soft_limit?: number
  hard_limit?: number
  currency?: string
  notify_at?: number[]
  enabled?: boolean
}

export interface SpendEntryDto {
  id: string
  budget_id: string
  virtual_key_id: string
  tenant_id: string
  model: string
  cost: number
  tokens: number
  created_at: string
}

export interface BudgetListResponse {
  data: BudgetDto[]
}

export function listBudgets(): Promise<BudgetListResponse> {
  return apiFetch(BASE)
}

export interface BudgetPage {
  items: BudgetDto[]
  total: number
  page: number
  perPage: number
}

interface PaginatedEnvelope<T> {
  data: T[]
  pagination?: { page: number; per_page: number; total: number }
}

// @sk-task ui-production-readiness#T3.2: Server-side paging/search (AC-008)
export async function listBudgetsPage(opts: { page?: number; perPage?: number; search?: string } = {}): Promise<BudgetPage> {
  const perPage = opts.perPage ?? 20
  const page = opts.page ?? 1
  const params = new URLSearchParams({ limit: String(perPage), offset: String((page - 1) * perPage) })
  if (opts.search) params.set('search', opts.search)
  const body = await apiFetch<PaginatedEnvelope<BudgetDto>>(`${BASE}?${params.toString()}`, { raw: true })
  return { items: body.data ?? [], total: body.pagination?.total ?? body.data?.length ?? 0, page, perPage }
}

export function listBudgetsByTenant(slug: string): Promise<BudgetListResponse> {
  return apiFetch(`${BASE}/tenants/${encodeURIComponent(slug)}`)
}

export function createBudget(req: CreateBudgetRequest): Promise<BudgetDto> {
  return apiFetch(BASE, { method: 'POST', body: req })
}

export function updateBudget(id: string, req: UpdateBudgetRequest): Promise<BudgetDto> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}`, { method: 'PATCH', body: req })
}

export function deleteBudget(id: string): Promise<void> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function budgetHistory(id: string): Promise<{ data: SpendEntryDto[] }> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}/history`)
}
