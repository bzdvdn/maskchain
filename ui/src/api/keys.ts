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

export function createKey(req: CreateKeyRequest): Promise<CreateKeyResponse> {
  return apiFetch(BASE, { method: 'POST', body: req })
}

export function updateKey(id: string, req: UpdateKeyRequest): Promise<VirtualKeyDto> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}`, { method: 'PATCH', body: req })
}

export function deleteKey(id: string): Promise<void> {
  return apiFetch(`${BASE}/${encodeURIComponent(id)}`, { method: 'DELETE' })
}