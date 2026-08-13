// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { TenantForm } from './TenantForm'

const { mockCreate, mockUpdate, mockGet, mockNavigate } = vi.hoisted(() => ({
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockGet: vi.fn(),
  mockNavigate: vi.fn(),
}))

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ slug: undefined }),
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}))

vi.mock('../../api/tenants', () => ({
  createTenant: (...args: unknown[]) => mockCreate(...args),
  updateTenant: (...args: unknown[]) => mockUpdate(...args),
  getTenant: (...args: unknown[]) => mockGet(...args),
}))

vi.mock('../../components/Toast', () => ({
  useToast: () => ({ toast: vi.fn() }),
}))

beforeEach(() => {
  vi.clearAllMocks()
})

describe('TenantForm retention mode', () => {
  it('sends retention_mode on create', async () => {
    mockCreate.mockResolvedValue({
      slug: 'acme',
      name: 'Acme',
      auth_header: 'Authorization',
      api_keys: ['sk-1'],
      retention_mode: 'meta',
      created_at: '2026-08-10T10:00:00Z',
      updated_at: '2026-08-10T10:00:00Z',
    })

    render(<TenantForm />)

    fireEvent.change(screen.getByLabelText(/Name \*/), { target: { value: 'Acme' } })
    fireEvent.change(screen.getByLabelText(/Slug \*/), { target: { value: 'acme' } })
    fireEvent.change(screen.getByLabelText(/API Keys/), { target: { value: 'sk-1' } })

    const select = screen.getByLabelText(/Retention Mode/)
    expect(select).toBeTruthy()
    fireEvent.change(select, { target: { value: 'meta' } })

    fireEvent.click(screen.getByText('Save'))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(expect.objectContaining({ retention_mode: 'meta' }))
    })
  })
})
