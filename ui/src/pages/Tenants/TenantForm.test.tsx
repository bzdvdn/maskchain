// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { TenantForm } from './TenantForm'

const { mockCreate, mockUpdate, mockGet, mockNavigate, mockSlug } = vi.hoisted(() => ({
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockGet: vi.fn(),
  mockNavigate: vi.fn(),
  mockSlug: { value: undefined as string | undefined },
}))

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ slug: mockSlug.value }),
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
  mockSlug.value = undefined
})

describe('TenantForm retention mode', () => {
  it('sends retention_mode on create', async () => {
    mockCreate.mockResolvedValue({
      slug: 'acme',
      name: 'Acme',
      auth_header: 'Authorization',
      retention_mode: 'meta',
      created_at: '2026-08-10T10:00:00Z',
      updated_at: '2026-08-10T10:00:00Z',
    })

    render(<TenantForm />)

    fireEvent.change(screen.getByLabelText(/Name \*/), { target: { value: 'Acme' } })
    fireEvent.change(screen.getByLabelText(/Slug \*/), { target: { value: 'acme' } })

    const select = screen.getByLabelText(/Retention Mode/)
    expect(select).toBeTruthy()
    fireEvent.change(select, { target: { value: 'meta' } })

    fireEvent.click(screen.getByText('Save'))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(expect.objectContaining({ retention_mode: 'meta' }))
    })
  })
})

// @sk-test ui-dictionary-editor#T3.2: create includes dictionaries (AC-001)
describe('TenantForm dictionaries (create)', () => {
  it('sends the added dictionary on create', async () => {
    mockCreate.mockResolvedValue({
      slug: 'acme',
      name: 'Acme',
      auth_header: 'Authorization',
      created_at: '2026-08-10T10:00:00Z',
      updated_at: '2026-08-10T10:00:00Z',
    })

    render(<TenantForm />)
    fireEvent.change(screen.getByLabelText(/Name \*/), { target: { value: 'Acme' } })
    fireEvent.change(screen.getByLabelText(/Slug \*/), { target: { value: 'acme' } })
    fireEvent.click(screen.getByText('+ Add dictionary'))
    fireEvent.change(screen.getByLabelText('Dictionary 1 name'), { target: { value: 'clients' } })
    fireEvent.change(screen.getByLabelText('Dictionary 1 entries'), { target: { value: 'alice\nbob' } })
    fireEvent.click(screen.getByText('Save'))

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          dictionaries: [{ name: 'clients', entries: ['alice', 'bob'], match_mode: 'exact' }],
        }),
      )
    })
  })

  // @sk-test ui-dictionary-editor#T3.2: validation blocks empty dictionary (AC-004)
  it('blocks save when a dictionary has no entries', async () => {
    render(<TenantForm />)
    fireEvent.change(screen.getByLabelText(/Name \*/), { target: { value: 'Acme' } })
    fireEvent.change(screen.getByLabelText(/Slug \*/), { target: { value: 'acme' } })
    fireEvent.click(screen.getByText('+ Add dictionary'))
    fireEvent.change(screen.getByLabelText('Dictionary 1 name'), { target: { value: 'clients' } })
    fireEvent.click(screen.getByText('Save'))

    expect(await screen.findByText('At least one entry is required')).toBeTruthy()
    expect(mockCreate).not.toHaveBeenCalled()
  })
})

// @sk-test ui-dictionary-editor#T3.2: edit persists changes and removals (AC-002, AC-005)
describe('TenantForm dictionaries (edit)', () => {
  const tenant = {
    slug: 'acme',
    name: 'Acme',
    auth_header: 'Authorization',
    retention_mode: 'full' as const,
    created_at: '',
    updated_at: '',
  }

  it('persists an edited dictionary', async () => {
    mockSlug.value = 'acme'
    mockGet.mockResolvedValue({
      ...tenant,
      dictionaries: [{ name: 'clients', entries: ['alice'], match_mode: 'exact' }],
    })
    mockUpdate.mockResolvedValue({})

    render(<TenantForm />)
    const nameInput = await screen.findByLabelText('Dictionary 1 name')
    expect((nameInput as HTMLInputElement).value).toBe('clients')
    fireEvent.change(nameInput, { target: { value: 'sites' } })
    fireEvent.click(screen.getByText('Save'))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        'acme',
        expect.objectContaining({
          dictionaries: [{ name: 'sites', entries: ['alice'], match_mode: 'exact' }],
        }),
      )
    })
  })

  it('drops a removed dictionary from the payload', async () => {
    mockSlug.value = 'acme'
    mockGet.mockResolvedValue({
      ...tenant,
      dictionaries: [
        { name: 'clients', entries: ['alice'], match_mode: 'exact' },
        { name: 'sites', entries: ['acme.com'], match_mode: 'contains' },
      ],
    })
    mockUpdate.mockResolvedValue({})

    render(<TenantForm />)
    fireEvent.click(await screen.findByLabelText('Remove dictionary 1'))
    fireEvent.click(screen.getByText('Save'))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        'acme',
        expect.objectContaining({
          dictionaries: [{ name: 'sites', entries: ['acme.com'], match_mode: 'contains' }],
        }),
      )
    })
  })
})
