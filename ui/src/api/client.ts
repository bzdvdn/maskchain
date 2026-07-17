import { getAdminToken } from './admin'

export class ApiError extends Error {
  status: number
  body: any
  constructor(status: number, body: any) {
    super(`API error ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

export class UnauthorizedError extends Error {
  constructor() {
    super('Unauthorized')
    this.name = 'UnauthorizedError'
  }
}

export class NotFoundError extends Error {
  constructor(resource: string) {
    super(`Not found: ${resource}`)
    this.name = 'NotFoundError'
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  headers?: Record<string, string>
}

export async function apiFetch<T>(url: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {
    ...opts.headers,
  }

  const token = getAdminToken()
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(url, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
    credentials: 'include',
  })

  if (res.status === 401) {
    throw new UnauthorizedError()
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    if (res.status === 404) {
      throw new NotFoundError(url)
    }
    throw new ApiError(res.status, body)
  }

  const body = await res.json()
  return body.data ?? body
}
