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
