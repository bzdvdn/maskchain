const BASE = '/api/v1/profiles'

export interface ProfileListItem {
  slug: string
  name: string
  status: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

export interface DictionaryDTO {
  name: string
  entries: string[]
  match_mode: 'exact' | 'contains' | 'regex' | 'fuzzy'
}

export interface PreprocessorDef {
  name: string
  type: string
  rules: { columns?: string[]; path?: string; mask: 'full' | 'surname' }[]
}

export interface ProfileResponse {
  id: string
  slug: string
  name: string
  description?: string
  status: string
  dictionaries?: DictionaryDTO[]
  preprocessors?: PreprocessorDef[]
  created_at: string
  updated_at: string
}

export interface CreateProfileRequest {
  slug: string
  name: string
  description?: string
  dictionaries?: DictionaryDTO[]
  preprocessors?: PreprocessorDef[]
}

export interface UpdateProfileRequest {
  name?: string
  description?: string
  dictionaries?: DictionaryDTO[]
  preprocessors?: PreprocessorDef[]
}

export interface ErrorResponse {
  error: string
  code: string
  details?: { field: string; message: string }[]
}

export async function listProfiles(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<ProfileListItem>> {
  const res = await fetch(`${BASE}?page=${page}&page_size=${pageSize}`)
  if (!res.ok) {
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  return res.json()
}

export async function getProfile(slug: string): Promise<ProfileResponse> {
  const res = await fetch(`${BASE}/${encodeURIComponent(slug)}`)
  if (!res.ok) {
    if (res.status === 404) throw new NotFoundError(slug)
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  return res.json()
}

export async function createProfile(
  req: CreateProfileRequest
): Promise<ProfileResponse> {
  const res = await fetch(BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
  return res.json()
}

export async function updateProfile(
  slug: string,
  req: UpdateProfileRequest
): Promise<ProfileResponse> {
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
  return res.json()
}

export async function deleteProfile(slug: string): Promise<void> {
  const res = await fetch(`${BASE}/${encodeURIComponent(slug)}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    if (res.status === 404) throw new NotFoundError(slug)
    const err: ErrorResponse = await res.json()
    throw new ApiError(res.status, err)
  }
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
    super(`Profile "${slug}" not found`)
    this.name = 'NotFoundError'
    this.slug = slug
  }
}
