import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  listProfiles,
  getProfile,
  createProfile,
  deleteProfile,
  ApiError,
  NotFoundError,
} from '../api/profiles'

const mockFetch = vi.fn()
global.fetch = mockFetch

beforeEach(() => {
  mockFetch.mockReset()
})

// @sk-test 41-profiles-ui#T5.2: API client tests (AC-002, AC-003, AC-004, AC-010)
describe('listProfiles', () => {
  it('returns paginated response on success', async () => {
    const expected = { data: [], total: 0, page: 1, page_size: 20 }
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(expected),
    })

    const result = await listProfiles(1, 20)
    expect(result).toEqual(expected)
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/profiles?page=1&page_size=20')
  })

  it('throws ApiError on failure', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: 'server error', code: 'INTERNAL' }),
    })

    await expect(listProfiles()).rejects.toThrow(ApiError)
  })
})

describe('createProfile', () => {
  it('sends POST request with body', async () => {
    const body = { slug: 'test', name: 'Test' }
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 201,
      json: () => Promise.resolve({ slug: 'test', name: 'Test' }),
    })

    const result = await createProfile(body)
    expect(result.name).toBe('Test')
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/profiles', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  })
})

describe('getProfile', () => {
  it('throws NotFoundError on 404', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'not found', code: 'NOT_FOUND' }),
    })

    await expect(getProfile('missing')).rejects.toThrow(NotFoundError)
  })
})

describe('deleteProfile', () => {
  it('sends DELETE request', async () => {
    mockFetch.mockResolvedValueOnce({ ok: true, status: 204 })

    await deleteProfile('test')
    expect(mockFetch).toHaveBeenCalledWith('/api/v1/profiles/test', {
      method: 'DELETE',
    })
  })
})
