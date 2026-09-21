// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Routing } from './Routing'

vi.mock('../api/routing', () => ({
  GLOBAL_TENANT: '*',
  listProviders: vi.fn(),
  listRoutes: vi.fn(),
  listModels: vi.fn(),
  listCostRates: vi.fn(),
  deleteProvider: vi.fn(),
  deleteRoute: vi.fn(),
  deleteCostRate: vi.fn(),
  upsertProvider: vi.fn(),
  upsertRoute: vi.fn(),
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

import { listProviders, listRoutes, listModels } from '../api/routing'

const mockProviders = vi.mocked(listProviders)
const mockRoutes = vi.mocked(listRoutes)
const mockModels = vi.mocked(listModels)

beforeEach(() => {
  vi.clearAllMocks()
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
