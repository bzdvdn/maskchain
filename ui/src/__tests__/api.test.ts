// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { listConversations, getConversation } from '../api/conversations'
import { setAdminToken } from '../api/admin'
import { UnauthorizedError } from '../api/client'

const mockFetch = vi.fn()
global.fetch = mockFetch

beforeEach(() => {
  mockFetch.mockReset()
  setAdminToken('test-token')
})

afterEach(() => {
  setAdminToken(null)
})

// @sk-task ui-production-readiness#T1.1: conversations use the shared client (AC-001)
describe('listConversations', () => {
  it('sends the admin token via the shared client', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { items: [] },
          pagination: { page: 1, per_page: 20, total: 0 },
        }),
    })

    const result = await listConversations(1, 20)
    expect(result.pagination.total).toBe(0)
    const [url, init] = mockFetch.mock.calls[0]
    expect(url).toBe('/api/v1/conversations?page=1&per_page=20')
    expect(init.headers.Authorization).toBe('Bearer test-token')
    expect(init.credentials).toBe('include')
  })

  it('dispatches the unauthorized event on 401', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      json: () => Promise.resolve({ error: 'unauthorized' }),
    })
    const onUnauthorized = vi.fn()
    window.addEventListener('maskchain:unauthorized', onUnauthorized)

    await expect(listConversations()).rejects.toThrow(UnauthorizedError)
    expect(onUnauthorized).toHaveBeenCalled()
    window.removeEventListener('maskchain:unauthorized', onUnauthorized)
  })
})

describe('getConversation', () => {
  it('unwraps the data envelope', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { id: 'c1', tenant_id: 't', model: 'm', status: 'ok', masked: false, streamed: false, created_at: '', payload: { request: '', response: null, masking: null } },
        }),
    })

    const result = await getConversation('c1')
    expect(result.id).toBe('c1')
  })
})
