// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Dashboard } from './Dashboard'

vi.mock('../api/analytics', () => ({
  getAnalyticsTokens: vi.fn(),
  getAnalyticsCost: vi.fn(),
  getAnalyticsSeries: vi.fn(),
}))
vi.mock('../api/budgets', () => ({
  listBudgets: vi.fn(),
  listBudgetsByTenant: vi.fn(),
}))
vi.mock('../api/routing', () => ({
  listProviders: vi.fn(),
}))
vi.mock('../api/keys', () => ({
  listKeys: vi.fn(),
  listKeysByTenant: vi.fn(),
}))
vi.mock('../api/conversations', () => ({
  listConversations: vi.fn(),
}))

import { getAnalyticsTokens, getAnalyticsCost, getAnalyticsSeries } from '../api/analytics'
import { listBudgets } from '../api/budgets'
import { listProviders } from '../api/routing'
import { listKeys } from '../api/keys'
import { listConversations } from '../api/conversations'

const mockTokens = vi.mocked(getAnalyticsTokens)
const mockCost = vi.mocked(getAnalyticsCost)
const mockSeries = vi.mocked(getAnalyticsSeries)
const mockBudgets = vi.mocked(listBudgets)
const mockProviders = vi.mocked(listProviders)
const mockKeys = vi.mocked(listKeys)
const mockConversations = vi.mocked(listConversations)

const seriesPoint = {
  bucket: '2026-08-30T10:00:00Z',
  input_tokens: 100,
  output_tokens: 50,
  cost: 1.2,
  requests: 30,
}

function seedEmpty() {
  mockTokens.mockResolvedValue({ records: [], totals: { total_input_tokens: 0, total_output_tokens: 0 } })
  mockCost.mockResolvedValue({ records: [], totals: { total_cost: 0, request_count: 0 } })
  mockSeries.mockResolvedValue({ series: [seriesPoint] })
  mockBudgets.mockResolvedValue({ data: [] })
  mockProviders.mockResolvedValue([])
  mockKeys.mockResolvedValue({ data: [] })
  mockConversations.mockResolvedValue({ items: [], pagination: { page: 1, per_page: 20, total: 0 } as never })
}

beforeEach(() => {
  vi.clearAllMocks()
  seedEmpty()
})

// @sk-test ui-v2-console#T3.4: metric toggle keeps the same range (AC-004)
describe('Dashboard metric toggle', () => {
  it('re-issues a series request with the same range when the metric changes', async () => {
    render(<MemoryRouter><Dashboard /></MemoryRouter>)

    await screen.findByText('Traffic trend')
    expect(mockSeries).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Cost' }))

    await waitFor(() => expect(mockSeries).toHaveBeenCalledTimes(2))
    expect(mockSeries).toHaveBeenLastCalledWith('', '', '')
  })
})

// @sk-test ui-v2-console#T3.4: attention panel maps budget + provider signals (AC-003)
describe('Dashboard needs attention', () => {
  it('renders budget exhaustion and provider-down items', async () => {
    mockBudgets.mockResolvedValue({
      data: [{ tenant_id: 'fintech', spent: 500, hard_limit: 500, type: 'monthly' } as never],
    })
    mockProviders.mockResolvedValue([{ name: 'anthropic-claude', status: 'down' } as never])

    render(<MemoryRouter><Dashboard /></MemoryRouter>)

    expect(await screen.findByText('Budget exhausted')).toBeTruthy()
    expect(await screen.findByText('Provider down')).toBeTruthy()
  })

  it('shows a nominal state when no signals exist', async () => {
    render(<MemoryRouter><Dashboard /></MemoryRouter>)

    expect(await screen.findByText('All systems nominal.')).toBeTruthy()
  })
})