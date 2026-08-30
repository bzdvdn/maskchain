// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Keys } from './Keys'

vi.mock('../api/keys', () => ({
  listKeys: vi.fn(),
  createKey: vi.fn(),
  updateKey: vi.fn(),
  deleteKey: vi.fn(),
}))
vi.mock('../api/tenants', () => ({
  listTenants: vi.fn(),
}))
vi.mock('../components/Toast', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../components/Toast')>()
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useToast: () => ({ toast: vi.fn() }),
  }
})

import { listKeys, createKey } from '../api/keys'
import { listTenants } from '../api/tenants'

const mockList = vi.mocked(listKeys)
const mockCreate = vi.mocked(createKey)
const mockTenants = vi.mocked(listTenants)

function keyDto(over: Record<string, unknown>) {
  return {
    id: 'key-1',
    tenant_id: 'acme',
    label: 'web-chat',
    allowed_models: ['gpt-4o'],
    blocked_models: [],
    spent: 10,
    enabled: true,
    created_at: '2026-08-01T10:00:00Z',
    expires_at: '2026-10-01T10:00:00Z',
    ...over,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  mockTenants.mockResolvedValue([{ slug: 'acme', name: 'Acme Corp' } as never])
  mockList.mockResolvedValue({ data: [keyDto({})] as never })
})

function renderKeys(entry = '/keys') {
  return render(<MemoryRouter initialEntries={[entry]}><Keys /></MemoryRouter>)
}

// @sk-test ui-v2-console#T3.4: create form validates and reveals the key once (AC-005)
describe('Keys create drawer', () => {
  it('opens via the create=1 query param and validates tenant', async () => {
    mockTenants.mockResolvedValue([])
    renderKeys('/keys?create=1')

    expect(await screen.findByRole('heading', { name: 'Create key' })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Create' }))
    expect(screen.getByText('Tenant is required')).toBeTruthy()
    expect(mockCreate).not.toHaveBeenCalled()
  })

  it('creates a key and reveals it exactly once', async () => {
    mockCreate.mockResolvedValue({ ...keyDto({}), key: 'sk-mc-raw-secret' } as never)
    renderKeys('/keys?create=1')

    await screen.findByRole('heading', { name: 'Create key' })
    fireEvent.change(screen.getByPlaceholderText('e.g. payments-gw, ci-pipeline'), { target: { value: 'payments-gw' } })

    fireEvent.click(screen.getByRole('button', { name: 'Create' }))

    await waitFor(() => expect(mockCreate).toHaveBeenCalled())
    expect(await screen.findByText('Key created')).toBeTruthy()
    expect(screen.getByText('sk-mc-raw-secret')).toBeTruthy()
    expect(mockCreate).toHaveBeenCalledWith(expect.objectContaining({ tenant_id: 'acme', label: 'payments-gw' }))
  })
})

// @sk-test ui-v2-console#T3.4: toolbar narrows rows by search (AC-006)
describe('Keys search filter', () => {
  it('filters rows by search text', async () => {
    mockList.mockResolvedValue({
      data: [
        keyDto({ id: 'k1', tenant_id: 'acme', label: 'web-chat' }),
        keyDto({ id: 'k2', tenant_id: 'fintech', label: 'payments-gw' }),
      ] as never,
    })
    renderKeys()

    expect(await screen.findByText('payments-gw')).toBeTruthy()

    fireEvent.change(screen.getByPlaceholderText('Search by label, tenant or model…'), { target: { value: 'web-chat' } })

    await waitFor(() => {
      expect(screen.queryByText('payments-gw')).toBeNull()
    })
    expect(screen.getByText('web-chat')).toBeTruthy()
  })
})

// @sk-test ui-v2-console#T3.4: spend progress colours by budget threshold (AC-006)
describe('Keys spend progress', () => {
  it('marks rows near the hard limit as danger', async () => {
    mockList.mockResolvedValue({
      data: [keyDto({ id: 'k1', spent: 95, budget_cap: 100, label: 'near-limit' })] as never,
    })
    const { container } = renderKeys()

    await screen.findByText('near-limit')
    expect(container.querySelector('.mc-progress-fill.danger')).toBeTruthy()
  })
})