// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Models } from './Models'

vi.mock('../api/routing', () => ({
  GLOBAL_TENANT: '*',
  listModels: vi.fn(),
  listProviders: vi.fn(),
  upsertCostRate: vi.fn(),
  upsertRoute: vi.fn(),
  deleteCostRate: vi.fn(),
  deleteRoute: vi.fn(),
}))
vi.mock('../components/Toast', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../components/Toast')>()
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useToast: () => ({ toast: vi.fn() }),
  }
})

import { listModels, listProviders, upsertCostRate, upsertRoute, deleteCostRate, deleteRoute } from '../api/routing'

const mockModels = vi.mocked(listModels)
const mockProviders = vi.mocked(listProviders)
const mockUpsertRate = vi.mocked(upsertCostRate)
const mockUpsertRoute = vi.mocked(upsertRoute)
const mockDeleteRate = vi.mocked(deleteCostRate)
const mockDeleteRoute = vi.mocked(deleteRoute)

beforeEach(() => {
  vi.clearAllMocks()
  mockProviders.mockResolvedValue([
    { name: 'openai', api_type: 'openai', base_url: 'https://api.openai.com/v1' },
    { name: 'anthropic', api_type: 'anthropic', base_url: 'https://api.anthropic.com' },
  ])
  mockModels.mockResolvedValue([
    { model: 'gpt-4o', input_price_per_1k: 1.5, output_price_per_1k: 2.5, currency: 'USD', default_providers: ['openai'], override_count: 2 },
  ])
})

// @sk-test routing-ia#T4.3: Models page shows cost, defaults and overrides (AC-006)
describe('Models page', () => {
  it('renders cost, default providers and override count', async () => {
    render(<Models />)
    expect(await screen.findByText('gpt-4o')).toBeTruthy()
    expect(screen.getByText('openai')).toBeTruthy()
    expect(screen.getByText('2')).toBeTruthy()
    expect(screen.getByText('1.5')).toBeTruthy()
  })

  it('saves cost and the global default route together', async () => {
    render(<Models />)
    fireEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    fireEvent.click(screen.getByRole('button', { name: /anthropic/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockUpsertRate).toHaveBeenCalledTimes(1))
    expect(mockUpsertRoute).toHaveBeenCalledWith({ tenant: '*', model: 'gpt-4o', providers: ['openai', 'anthropic'] })
  })

  it('deletes the cost rate and the global route', async () => {
    render(<Models />)
    fireEvent.click(await screen.findByRole('button', { name: 'Delete' }))
    // the confirm modal adds a second Delete button; click the modal's
    const deletes = screen.getAllByRole('button', { name: 'Delete' })
    fireEvent.click(deletes[deletes.length - 1])

    await waitFor(() => expect(mockDeleteRate).toHaveBeenCalledWith('gpt-4o'))
    expect(mockDeleteRoute).toHaveBeenCalledWith({ tenant: '*', model: 'gpt-4o', providers: [] })
  })
})
