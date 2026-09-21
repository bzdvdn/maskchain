import { apiFetch } from './client'

const BASE = '/api/v1/sessions'

export interface Session {
  session_id: string
  tenant_id: string
  model: string
  status: string
  created_at: string
  expires_at: string
  token_count: number
}

interface ListResponse {
  items: Session[]
  total: number
  page: number
  limit: number
}

export async function listSessions(page = 1, limit = 100): Promise<Session[]> {
  const res = await apiFetch<ListResponse>(`${BASE}?page=${page}&limit=${limit}`)
  return res?.items ?? []
}

export function closeSession(id: string): Promise<void> {
  return apiFetch<void>(`${BASE}/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function extendSession(id: string, ttlSeconds = 1800): Promise<void> {
  return apiFetch<void>(`${BASE}/${encodeURIComponent(id)}/extend`, {
    method: 'PATCH',
    body: { ttl_seconds: ttlSeconds },
  })
}
