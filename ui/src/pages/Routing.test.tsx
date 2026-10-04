// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Routing } from './Routing'

vi.mock('../api/routing', () => ({
  GLOBAL_TENANT: '*',
  listProviders: vi.fn(),
  listRoutes: vi.fn(),
  listModels: vi.fn(),
  listAliases: vi.fn(),
  listCostRates: vi.fn(),
  deleteProvider: vi.fn(),
  deleteRoute: vi.fn(),
  deleteAlias: vi.fn(),
  deleteCostRate: vi.fn(),
  upsertProvider: vi.fn(),
  upsertRoute: vi.fn(),
  upsertAlias: vi.fn(),
  upsertCostRate: vi.fn(),
  isMaskedKey: (v?: string) => !!v && v.includes('***'),
}))
vi.mock('../components/Toast', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../components/Toast')>()
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useToast: () => ({ toast: vi.fn() }),
  }
})

import { listProviders, listRoutes, listModels, listAliases, upsertAlias } from '../api/routing'

const mockProviders = vi.mocked(listProviders)
const mockRoutes = vi.mocked(listRoutes)
const mockModels = vi.mocked(listModels)
const mockAliases = vi.mocked(listAliases)
const mockUpsertAlias = vi.mocked(upsertAlias)

beforeEach(() => {
  vi.clearAllMocks()
  mockAliases.mockResolvedValue([])
  mockProviders.mockResolvedValue([
    { name: 'openai', api_type: 'openai', base_url: 'https://api.openai.com/v1' },
    { name: 'anthropic', api_type: 'anthropic', base_url: 'https://api.anthropic.com' },
    { name: 'ollama', api_type: 'ollama', base_url: 'http://localhost:11434' },
  ])
  mockRoutes.mockResolvedValue([
    { tenant: 'acme', model: 'gpt-4o', providers: ['anthropic', 'openai'] },
    { tenant: '*', model: 'gpt-4o', providers: ['openai'] },
    { tenant: '*', model: 'llama3.2', providers: ['ollama'] },
  ])
  mockModels.mockResolvedValue([
    { model: 'gpt-4o', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD', default_providers: ['openai'], override_count: 1 },
    { model: 'llama3.2', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD', default_providers: ['ollama'], override_count: 0 },
  ])
})

// @sk-test routing-ia#T4.3: tenant overrides and inherited defaults (AC-007)
describe('Routing overrides', () => {
  it('lists tenant overrides and shows models inheriting the global default', async () => {
    render(<MemoryRouter><Routing /></MemoryRouter>)

    // override row for the acme tenant, with the global marker not shown as a tenant
    expect(await screen.findByText('acme')).toBeTruthy()
    expect(screen.queryByText('*')).toBeNull()
    expect(screen.getAllByText('gpt-4o').length).toBeGreaterThan(0)

    // the model without an override is shown as inherited
    expect(screen.getByText('inherited')).toBeTruthy()
    expect(screen.getByText('llama3.2')).toBeTruthy()
  })
})

// @sk-test model-aliases-weighted-lb#T3.5: adding an alias calls the admin API (AC-009)
describe('Routing aliases', () => {
  it('adds a tenant alias', async () => {
    mockUpsertAlias.mockResolvedValue({ tenant: 'default', alias: 'alias-model', target: 'target-model' })
    render(<MemoryRouter><Routing /></MemoryRouter>)

    const addButtons = await screen.findAllByRole('button', { name: 'Add Alias' })
    fireEvent.click(addButtons[0])
    fireEvent.change(screen.getByPlaceholderText('gpt-4o'), { target: { value: 'alias-model' } })
    fireEvent.change(screen.getByPlaceholderText('openai/gpt-4o-2024'), { target: { value: 'target-model' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockUpsertAlias).toHaveBeenCalledTimes(1))
    expect(mockUpsertAlias.mock.calls[0][0]).toMatchObject({ alias: 'alias-model', target: 'target-model' })
  })
})
