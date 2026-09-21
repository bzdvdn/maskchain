import { apiFetch } from './client'

const BASE = '/api/v1/keys'

export interface VirtualKeyDto {
  id: string
  tenant_id: string
  label: string
  allowed_models: string[]
  blocked_models: string[]
  budget_cap?: number
  spent: number
  expires_at?: string | null
  metadata?: Record<string, string>
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateKeyRequest {
  tenant_id: string
  label?: string
  allowed_models?: string[]
  blocked_models?: string[]
  budget_cap?: number
  expires_at?: string | null
  metadata?: Record<string, string>
}

export interface UpdateKeyRequest {
  label?: string
  allowed_models?: string[]
  blocked_models?: string[]
  budget_cap?: number
  expires_at?: string | null
  enabled?: boolean
  metadata?: Record<string, string>
}

export interface CreateKeyResponse extends VirtualKeyDto {
  key: string
}

export interface ListResponse {
  data: VirtualKeyDto[]
}

export function listKeys(): Promise<ListResponse> {
  return apiFetch(BASE)
}

export interface KeyPage {
  items: VirtualKeyDto[]
  total: number
  page: number
  perPage: number
}

interface PaginatedEnvelope<T> {
  data: T[]
  pagination?: { page: number; per_page: number; total: number }
}

// @sk-task ui-production-readiness#T3.2: Server-side paging/search (AC-008)
export async function listKeysPage(opts: { page?: number; perPage?: number; search?: string } = {}): Promise<KeyPage> {
  const perPage = opts.perPage ?? 20
  const page = opts.page ?? 1
  const params = new URLSearchParams({ limit: String(perPage), offset: String((page - 1) * perPage) })
  if (opts.search) params.set('search', opts.search)
  const body = await apiFetch<PaginatedEnvelope<VirtualKeyDto>>(`${BASE}?${params.toString()}`, { raw: true })
  return { items: body.data ?? [], total: body.pagination?.total ?? body.data?.length ?? 0, page, perPage }
}

export function createKey(req: CreateKeyRequest): Promise<CreateKeyResponse> {
  return apiFetch(BASE, { method: 'POST', body: req })
}

export function updateKey(id: string, req: UpdateKeyRequest): Promise<VirtualKeyDto> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}`, { method: 'PATCH', body: req })
}

export function deleteKey(id: string): Promise<void> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// rotateKey issues a new secret for an existing key; the plaintext is returned once.
export function rotateKey(id: string): Promise<CreateKeyResponse> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}/rotate`, { method: 'POST' })
}