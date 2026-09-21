// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Conversations } from './Conversations'

function renderConversations() {
  return render(<MemoryRouter><Conversations /></MemoryRouter>)
}
import type {
  ConversationDetail,
  ConversationListItem,
} from '../api/conversations'
import { decodeBase64Utf8 } from '../utils/base64'

vi.mock('../api/conversations', () => ({
  listConversations: vi.fn(),
  getConversation: vi.fn(),
}))

import { listConversations, getConversation } from '../api/conversations'

const mockList = vi.mocked(listConversations)
const mockGet = vi.mocked(getConversation)

function b64(s: string) {
  return btoa(unescape(encodeURIComponent(s)))
}

function listItem(over: Partial<ConversationListItem>): ConversationListItem {
  return {
    id: 'id-1',
    tenant_id: 'tenant-a',
    model: 'gpt-4o',
    status: 'ok',
    masked: true,
    streamed: false,
    created_at: '2026-08-10T10:00:00Z',
    ...over,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
})

// @sk-test conversation-logging#T4.2: list renders records metadata (AC-006)
describe('Conversations list', () => {
  it('renders list items from API', async () => {
    mockList.mockResolvedValueOnce({
      items: [listItem({}), listItem({ id: 'id-2', model: 'gpt-4o-mini', streamed: true, status: 'blocked' })],
      pagination: { page: 1, per_page: 20, total: 2 },
    })

    renderConversations()

    expect(await screen.findByText(/id-1/)).toBeTruthy()
    expect(screen.getAllByText('tenant-a').length).toBe(3)
    expect(screen.getAllByText('gpt-4o-mini').length).toBe(2)
    expect(screen.getAllByText('blocked').length).toBe(2)
    expect(mockList).toHaveBeenCalledWith(1, 20, {})
  })

  it('re-requests with masked filter when selected', async () => {
    mockList.mockResolvedValue({
      items: [listItem({})],
      pagination: { page: 1, per_page: 20, total: 1 },
    })

    renderConversations()

    await screen.findByText(/id-1/)

    const selects = screen.getAllByRole('combobox')
    fireEvent.change(selects[3], { target: { value: 'true' } })

    await waitFor(() => {
      expect(mockList).toHaveBeenLastCalledWith(1, 20, { masked: 'true' })
    })
    expect(screen.getByRole('combobox', { name: /Masked/ })).toBeTruthy()
  })

  it('shows empty state when no records', async () => {
    mockList.mockResolvedValueOnce({ items: [], pagination: { page: 1, per_page: 20, total: 0 } })

    renderConversations()

    expect(await screen.findByText('No conversations')).toBeTruthy()
  })
})

// @sk-test conversation-logging#T4.2: detail decodes base64 payload and masking (AC-005, AC-009)
describe('Conversations detail', () => {
  it('fetches and decodes detail on row click', async () => {
    mockList.mockResolvedValueOnce({
      items: [listItem({})],
      pagination: { page: 1, per_page: 20, total: 1 },
    })

    const reqPlain = '{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}'
    const respPlain = 'data: {"content":"hi"}\n\n'
    const maskingPlain = '[{"placeholder":"[MASK_A.0]","original":"secret@example.com"}]'
    const detail: ConversationDetail = {
      id: 'id-1',
      tenant_id: 'tenant-a',
      model: 'gpt-4o',
      status: 'ok',
      masked: true,
      streamed: true,
      mask_id: 'MASK_DETAIL_A',
      created_at: '2026-08-10T10:00:00Z',
      payload: {
        request: b64(reqPlain),
        response: b64(respPlain),
        masking: b64(maskingPlain),
      },
    }
    mockGet.mockResolvedValueOnce(detail)

    renderConversations()

    fireEvent.click(await screen.findByText(/id-1/))

    expect(await screen.findByText(reqPlain)).toBeTruthy()
    expect(screen.getByText(/data: \{"content":"hi"\}/)).toBeTruthy()
    expect(screen.getByText('secret@example.com')).toBeTruthy()
    expect(screen.getByText('[MASK_A.0]')).toBeTruthy()
    expect(screen.getByText('MASK_DETAIL_A')).toBeTruthy()
    expect(mockGet).toHaveBeenCalledWith('id-1')
  })

  it('shows error state when detail fetch fails', async () => {
    mockList.mockResolvedValueOnce({
      items: [listItem({})],
      pagination: { page: 1, per_page: 20, total: 1 },
    })
    mockGet.mockRejectedValueOnce(new Error('boom'))

    renderConversations()

    fireEvent.click(await screen.findByText(/id-1/))

    await waitFor(() => {
      expect(screen.getByText('Failed to load conversation detail')).toBeTruthy()
    })
  })

  it('highlights active row and collapses detail on second click', async () => {
    mockList.mockResolvedValueOnce({
      items: [listItem({})],
      pagination: { page: 1, per_page: 20, total: 1 },
    })
    const detail: ConversationDetail = {
      ...listItem({}),
      payload: {
        request: b64('{"model":"gpt-4o"}'),
        response: null,
        masking: null,
      },
    }
    mockGet.mockResolvedValueOnce(detail)

    renderConversations()

    const row = (await screen.findByText(/id-1/)).closest('tr')!
    fireEvent.click(row)
    expect(await screen.findByText('Collapse')).toBeTruthy()
    expect(row.className).toContain('row-active')

    fireEvent.click(row)
    await waitFor(() => {
      expect(screen.queryByText('Collapse')).toBeNull()
    })
    expect(row.className).not.toContain('row-active')
  })
})

// @sk-test conversation-logging#T4.2: decodeBase64Utf8 handles UTF-8 and masking (AC-009)
describe('decodeBase64Utf8', () => {
  it('round-trips UTF-8 content', () => {
    const src = '{"role":"user","content":"привет 👋"}'
    expect(decodeBase64Utf8(b64(src))).toBe(src)
  })
})
