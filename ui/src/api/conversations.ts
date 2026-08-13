const BASE = '/api/v1/conversations'

// @sk-task conversation-logging#T3.2: Conversation list item type (AC-006)
// @sk-task conversation-logging#T5.1: Expose mask_id in list item (AC-005)
export interface ConversationListItem {
  id: string
  tenant_id: string
  model: string
  status: string
  masked: boolean
  streamed: boolean
  mask_id?: string
  created_at: string
}

// @sk-task conversation-logging#T3.2: Conversation detail payload type (AC-005)
export interface ConversationPayload {
  request: string
  response: string | null
  masking: string | null
}

// @sk-task conversation-logging#T3.2: Conversation detail type (AC-005)
export interface ConversationDetail extends ConversationListItem {
  payload: ConversationPayload
}

interface ConversationListResult {
  items: ConversationListItem[]
  pagination: { page: number; per_page: number; total: number }
}

export interface ConversationFilters {
  tenant_id?: string
  status?: string
  model?: string
  masked?: 'true' | 'false'
}

// @sk-task conversation-logging#T3.2: listConversations fetches metadata-only list (AC-006)
export async function listConversations(
  page = 1,
  perPage = 20,
  filters: ConversationFilters = {},
): Promise<ConversationListResult> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  })
  if (filters.tenant_id) params.set('tenant_id', filters.tenant_id)
  if (filters.status) params.set('status', filters.status)
  if (filters.model) params.set('model', filters.model)
  if (filters.masked) params.set('masked', filters.masked)
  const res = await fetch(`${BASE}?${params.toString()}`, { credentials: 'include' })
  if (!res.ok) throw new Error('failed to load conversations')
  const body = await res.json()
  return {
    items: (body.data?.items ?? []) as ConversationListItem[],
    pagination: body.pagination ?? { page, per_page: perPage, total: 0 },
  }
}

// @sk-task conversation-logging#T3.2: getConversation fetches a single record with payload (AC-005)
export async function getConversation(id: string): Promise<ConversationDetail> {
  const res = await fetch(`${BASE}/${encodeURIComponent(id)}`, { credentials: 'include' })
  if (!res.ok) throw new Error('failed to load conversation')
  const body = await res.json()
  return body.data ?? body
}
