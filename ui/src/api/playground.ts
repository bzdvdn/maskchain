import { apiFetch } from './client'

const BASE = '/api/v1/admin/playground'

export interface PlaygroundMessage {
  role: 'system' | 'user' | 'assistant'
  content: string
}

export interface PlaygroundRequest {
  api_key: string
  model: string
  messages: PlaygroundMessage[]
}

export interface PlaygroundResponse {
  status: number
  body: unknown
  shield_status?: string | null
}

// sendPlayground routes a test chat completion through the gateway (server-side,
// so no cross-origin or secret exposure in the browser).
export function sendPlayground(req: PlaygroundRequest): Promise<PlaygroundResponse> {
  return apiFetch<PlaygroundResponse>(BASE, { method: 'POST', body: req })
}
