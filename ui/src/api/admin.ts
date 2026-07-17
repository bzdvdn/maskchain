const BASE = '/api/v1/admin'

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expires_at: number
}

const STORAGE_KEY = 'admin_token'

function loadToken(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

let _token: string | null = loadToken()

export function getAdminToken(): string | null {
  return _token
}

export function setAdminToken(token: string | null) {
  _token = token
  try {
    if (token) {
      localStorage.setItem(STORAGE_KEY, token)
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  } catch { /* localStorage unavailable */ }
}

export async function login(req: LoginRequest): Promise<LoginResponse> {
  const res = await fetch(`${BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
    credentials: 'include',
  })
  if (res.status === 401) {
    throw new Error('invalid credentials')
  }
  if (res.status === 503) {
    throw new Error('admin not configured')
  }
  if (!res.ok) {
    throw new Error('login failed')
  }
  const body = await res.json()
  const data = (body.data ?? body) as LoginResponse
  setAdminToken(data.token)
  return data
}

export async function logout(): Promise<void> {
  await fetch(`${BASE}/logout`, {
    method: 'POST',
    credentials: 'include',
  })
  setAdminToken(null)
}
