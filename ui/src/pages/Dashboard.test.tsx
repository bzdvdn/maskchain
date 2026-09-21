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

    await screen.findByText(/Traffic trend/)
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
// @sk-test operations-hq-charts#T4.3: each card plots its own metric (AC-001)
describe('Dashboard KPI sparklines', () => {
  it('draws a different sparkline per card metric', async () => {
    mockSeries.mockResolvedValue({
      series: [
        { bucket: '2026-08-30T00:00:00Z', input_tokens: 100, output_tokens: 0, cost: 10, requests: 1 },
        { bucket: '2026-08-31T00:00:00Z', input_tokens: 0, output_tokens: 100, cost: 1, requests: 10 },
      ],
    })

    const { container } = render(<MemoryRouter><Dashboard /></MemoryRouter>)
    await screen.findByText(/Traffic trend/)

    const paths = Array.from(container.querySelectorAll('.spark-svg path')).map((p) => p.getAttribute('d'))
    expect(paths.length).toBe(3)
    expect(new Set(paths).size).toBeGreaterThan(1)
  })
})

// @sk-test operations-hq-charts#T4.3: card labels reflect range and scope (AC-002)
describe('Dashboard labels', () => {
  it('labels cards with the selected range and all-tenants scope', async () => {
    render(<MemoryRouter><Dashboard /></MemoryRouter>)
    expect(await screen.findByText('Spend · 7d')).toBeTruthy()
    expect(screen.getByText('Pass rate · last 100 · all tenants')).toBeTruthy()
  })

  it('includes the workspace in labels and scopes pass rate', async () => {
    localStorage.setItem('maskchain.workspace', 'acme')
    render(<MemoryRouter><Dashboard /></MemoryRouter>)

    expect(await screen.findByText('Spend · 7d · acme')).toBeTruthy()
    await waitFor(() => expect(mockConversations).toHaveBeenCalledWith(1, 100, { tenant_id: 'acme' }))
    localStorage.clear()
  })
})

// @sk-test operations-hq-charts#T4.3: trend header states metric, range and total (AC-004)
describe('Dashboard trend header', () => {
  it('names the metric and range and updates on switch', async () => {
    render(<MemoryRouter><Dashboard /></MemoryRouter>)
    expect(await screen.findByText(/Traffic trend · Tokens · 7d/)).toBeTruthy()
    expect(screen.getByText(/Total:/)).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Cost' }))
    expect(await screen.findByText(/Traffic trend · Cost · 7d/)).toBeTruthy()
  })
})

// @sk-test operations-hq-charts#T4.3: requests delta is computed (AC-005)
describe('Dashboard requests delta', () => {
  it('renders a delta on the Requests card', async () => {
    mockCost
      .mockResolvedValueOnce({ records: [], totals: { total_cost: 10, request_count: 100 } } as never)
      .mockResolvedValueOnce({ records: [], totals: { total_cost: 5, request_count: 80 } } as never)

    render(<MemoryRouter><Dashboard /></MemoryRouter>)
    // requests: (100-80)/80 = +25% (distinct from the spend delta)
    expect(await screen.findByText(/▲ 25%/)).toBeTruthy()
  })
})

// @sk-test operations-hq-charts#T4.3: semantics and empty states are explicit (AC-007, AC-008)
describe('Dashboard semantics and empty states', () => {
  it('states the units and meaning of the values', async () => {
    render(<MemoryRouter><Dashboard /></MemoryRouter>)
    expect(await screen.findByText(/Tokens = input \+ output/)).toBeTruthy()
  })

  it('shows the chart empty state when the series is empty', async () => {
    mockTokens.mockResolvedValue({ records: [], totals: { total_input_tokens: 10, total_output_tokens: 5 } } as never)
    mockSeries.mockResolvedValue({ series: [] })
    render(<MemoryRouter><Dashboard /></MemoryRouter>)
    expect(await screen.findByText('No data for this period')).toBeTruthy()
  })

  it('shows a neutral state when the series is all zero', async () => {
    mockTokens.mockResolvedValue({ records: [], totals: { total_input_tokens: 10, total_output_tokens: 5 } } as never)
    mockSeries.mockResolvedValue({ series: [{ bucket: '2026-08-30T00:00:00Z', input_tokens: 0, output_tokens: 0, cost: 0, requests: 0 }] })
    render(<MemoryRouter><Dashboard /></MemoryRouter>)
    expect(await screen.findByText('No activity in this period')).toBeTruthy()
  })
})
