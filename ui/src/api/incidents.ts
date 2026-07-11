const BASE = '/api/v1/incidents'

export interface IncidentResponse {
  id: string
  request_id: string
  timestamp: string
  tenant: string
  profile_slug: string
  detector_type: string
  entry_value?: string
  severity: string
  action: string
  prompt_snippet_redacted?: string
  response_snippet?: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

export interface IncidentFilterParams {
  severity?: string
  tenant?: string
  profile_slug?: string
  page?: number
  page_size?: number
}

// @sk-task 60-audit-incidents#T3.1: Incident API client (AC-005)
export async function listIncidents(
  params: IncidentFilterParams = {}
): Promise<PaginatedResponse<IncidentResponse>> {
  const query = new URLSearchParams()
  if (params.severity) query.set('severity', params.severity)
  if (params.tenant) query.set('tenant', params.tenant)
  if (params.profile_slug) query.set('profile_slug', params.profile_slug)
  if (params.page) query.set('page', String(params.page))
  if (params.page_size) query.set('page_size', String(params.page_size))

  const qs = query.toString()
  const url = qs ? `${BASE}?${qs}` : BASE

  const res = await fetch(url)
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error || 'Failed to list incidents')
  }
  return res.json()
}

// @sk-task 60-audit-incidents#T3.1: Get single incident by ID (AC-005)
export async function getIncident(id: string): Promise<IncidentResponse> {
  const res = await fetch(`${BASE}/${encodeURIComponent(id)}`)
  if (!res.ok) {
    if (res.status === 404) throw new Error('Incident not found')
    const err = await res.json()
    throw new Error(err.error || 'Failed to get incident')
  }
  return res.json()
}

// @sk-task 60-audit-incidents#T3.1: Export incidents (AC-005)
export async function exportIncidents(
  params: IncidentFilterParams & { format: 'csv' | 'json' }
): Promise<Blob> {
  const query = new URLSearchParams()
  query.set('format', params.format)
  if (params.severity) query.set('severity', params.severity)
  if (params.tenant) query.set('tenant', params.tenant)
  if (params.profile_slug) query.set('profile_slug', params.profile_slug)

  const res = await fetch(`${BASE}/export?${query.toString()}`)
  if (!res.ok) {
    const err = await res.json()
    throw new Error(err.error || 'Failed to export incidents')
  }
  return res.blob()
}
