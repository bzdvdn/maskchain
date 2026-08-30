// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Routing } from './Routing'

vi.mock('../api/routing', () => ({
  listProviders: vi.fn(),
  listRoutes: vi.fn(),
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

import { listProviders, listRoutes, listCostRates } from '../api/routing'

const mockProviders = vi.mocked(listProviders)
const mockRoutes = vi.mocked(listRoutes)
const mockRates = vi.mocked(listCostRates)

beforeEach(() => {
  vi.clearAllMocks()
  mockRoutes.mockResolvedValue([{ tenant: 'acme', model: 'gpt-4o', providers: ['openai', 'anthropic'] }])
  mockRates.mockResolvedValue([])
})

// @sk-test ui-v2-console#T4.5: provider cards render three health states (AC-008)
describe('Routing provider cards', () => {
  it('renders healthy, down and degraded states', async () => {
    mockProviders.mockResolvedValue([
      { name: 'openai', api_type: 'openai', base_url: 'https://api.openai.com/v1', status: 'up', latency_ms: 12, last_check: Math.floor(Date.now() / 1000) - 5 },
      { name: 'anthropic', api_type: 'anthropic', base_url: 'https://api.anthropic.com', status: 'down' },
      { name: 'azure', api_type: 'proxy', base_url: 'https://azure.example', status: 'degraded', latency_ms: 890 },
    ])

    render(<Routing />)

    expect(await screen.findByText('up')).toBeTruthy()
    expect(screen.getByText('down')).toBeTruthy()
    expect(screen.getByText('degraded')).toBeTruthy()
    expect(screen.getAllByText('openai').length).toBeGreaterThan(0)
    expect(screen.getAllByText('12ms').length).toBeGreaterThan(0)
  })
})