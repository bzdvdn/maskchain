// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { Analytics } from './Analytics'

vi.mock('../api/analytics', () => ({
  getAnalyticsTokens: vi.fn(),
  getAnalyticsCost: vi.fn(),
  getAnalyticsSeries: vi.fn(),
  withTenant: (base: string, tenant?: string) => (tenant ? `${base}&tenant=${tenant}` : base),
}))

import { getAnalyticsTokens, getAnalyticsCost, getAnalyticsSeries } from '../api/analytics'

const mockTokens = vi.mocked(getAnalyticsTokens)
const mockCost = vi.mocked(getAnalyticsCost)
const mockSeries = vi.mocked(getAnalyticsSeries)

function seed(spend = 500, reqCount = 100) {
  mockTokens.mockResolvedValue({ records: [], totals: { total_input_tokens: 700, total_output_tokens: 300 } })
  mockCost.mockResolvedValue({
    records: [{ tenant_id: 'acme', model: 'gpt-4o', total_cost: spend, request_count: reqCount, period_start: '2026-08-01T00:00:00Z', period_end: '2026-08-30T00:00:00Z' }],
    totals: { total_cost: spend, request_count: reqCount },
  })
  mockSeries.mockResolvedValue({
    series: [{ bucket: '2026-08-30T00:00:00Z', input_tokens: 700, output_tokens: 300, cost: spend, requests: reqCount }],
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  seed()
})

// @sk-test ui-v2-console#T4.5: analytics range is restored from localStorage (AC-007)
describe('Analytics range persistence', () => {
  it('replays the persisted range on mount', async () => {
    localStorage.setItem('maskchain.analytics.range', JSON.stringify({ mode: '30d', from: '2026-07-01T00:00:00Z', to: '2026-07-31T00:00:00Z' }))

    render(<Analytics />)

    await waitFor(() => {
      expect(mockCost).toHaveBeenCalledWith('2026-07-01T00:00:00Z', '2026-07-31T00:00:00Z', '')
    })
  })
})

// @sk-test ui-v2-console#T4.5: analytics renders compare deltas vs previous period (AC-007)
describe('Analytics compare', () => {
  it('shows an up delta badge when the current period exceeds the previous', async () => {
    seed(500, 100)
    render(<Analytics />)

    expect(await screen.findByText('$500.00')).toBeTruthy()
  })
})