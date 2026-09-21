import { apiFetch } from './client'

const BASE = '/api/v1/audit'

export interface AuditEntry {
  id: number
  admin_username: string
  action: string
  target: string
  details: string
  created_at: string
}

interface ListResponse {
  items: AuditEntry[]
  total: number
}

export async function listAudit(limit = 100, offset = 0): Promise<AuditEntry[]> {
  const res = await apiFetch<ListResponse>(`${BASE}?limit=${limit}&offset=${offset}`)
  return res?.items ?? []
}
