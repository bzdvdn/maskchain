const BASE = '/api/v1/tenants'

export interface PIARule {
  label: string
  type: string
  pattern: string
  action: string
}

export interface PIIConfig {
  enabled: boolean
  default_action: string
  rules: PIARule[]
}

export interface DictionaryItem {
  name: string
  entries: any[]
  match_mode: string
}

export interface TenantListItem {
  slug: string
  name: string
  auth_header: string
  api_keys: string[]
  pii_config?: PIIConfig
  created_at: string
  updated_at: string
}

export interface TenantResponse {
  slug: string
  name: string
  auth_header: string
  api_keys: string[]
  dictionaries?: DictionaryItem[]
  pii_config?: PIIConfig
  created_at: string
  updated_at: string
}

export interface CreateTenantRequest {
  slug: string
  name: string
  auth_header?: string
  api_keys: string[]
  dictionaries?: DictionaryItem[]
  pii_config?: PIIConfig
}

export interface UpdateTenantRequest {
  name: string
  auth_header?: string
  api_keys: string[]
  dictionaries?: DictionaryItem[]
  pii_config?: PIIConfig
}

export interface DictionaryRequest {
  dictionaries: DictionaryItem[]
}

export interface DictionaryResponse {
  dictionaries: DictionaryItem[]
}

export interface ErrorResponse {
  error: string
  code: string
  details?: { field: string; message: string }[]
}

export async function listTenants(): Promise<TenantListItem[]> {
  const res = await fetch(BASE)
  if (!res.ok) {
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  const body = await res.json()
  return body.data ?? body
}

export async function getTenant(slug: string): Promise<TenantResponse> {
  const res = await fetch(`${BASE}/${encodeURIComponent(slug)}`)
  if (!res.ok) {
    if (res.status === 404) throw new NotFoundError(slug)
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  const body = await res.json()
  return body.data ?? body
}

export async function createTenant(req: CreateTenantRequest): Promise<TenantResponse> {
  const res = await fetch(BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  const body = await res.json()
  return body.data ?? body
}

export async function updateTenant(slug: string, req: UpdateTenantRequest): Promise<TenantResponse> {
  const res = await fetch(`${BASE}/${encodeURIComponent(slug)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    if (res.status === 404) throw new NotFoundError(slug)
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  const body = await res.json()
  return body.data ?? body
}

export async function deleteTenant(slug: string): Promise<void> {
  const res = await fetch(`${BASE}/${encodeURIComponent(slug)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    if (res.status === 404) throw new NotFoundError(slug)
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
}

export async function updateDictionaries(slug: string, req: DictionaryRequest): Promise<DictionaryResponse> {
  const res = await fetch(`${BASE}/${encodeURIComponent(slug)}/dictionaries`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  const body = await res.json()
  return body.data ?? body
}

export class ApiError extends Error {
  status: number
  body: ErrorResponse

  constructor(status: number, body: ErrorResponse) {
    super(body.error)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

export class NotFoundError extends Error {
  slug: string

  constructor(slug: string) {
    super(`Tenant "${slug}" not found`)
    this.name = 'NotFoundError'
    this.slug = slug
  }
}
