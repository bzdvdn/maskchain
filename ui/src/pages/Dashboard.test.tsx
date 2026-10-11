// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Dashboard } from './Dashboard'
import { AttentionProvider } from '../components/AttentionProvider'

vi.mock('../api/tenants', () => ({
  listTenants: vi.fn().mockResolvedValue([]),
  getComplianceReport: vi.fn(),
  applyCompliancePack: vi.fn(),
}))
vi.mock('../api/shield', () => ({
  getShieldCatalog: vi.fn().mockResolvedValue({ detectors: [], reactions: [], packs: [] }),
}))

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
  mockKeys.mockResolvedValue({ data: [{ id: 'k1', tenant_id: 'default', label: 'prod', allowed_models: [], blocked_models: [], spent: 0, enabled: true, created_at: '', updated_at: '' }] as never })
  mockConversations.mockResolvedValue({ items: [], pagination: { page: 1, per_page: 20, total: 0 } as never })
}

function renderDash() {
  return render(
    <MemoryRouter>
      <AttentionProvider>
        <Dashboard />
      </AttentionProvider>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  seedEmpty()
})

// @sk-test ui-v2-console#T3.4: metric toggle keeps the same range (AC-004)
describe('Dashboard metric toggle', () => {
  it('re-issues a series request with the same range when the metric changes', async () => {
    renderDash()

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

    renderDash()

    expect(await screen.findByText('Budget exhausted')).toBeTruthy()
    expect(await screen.findByText('Provider down')).toBeTruthy()
  })

  it('shows a nominal state when no signals exist', async () => {
    renderDash()

    expect(await screen.findByText('All systems nominal.')).toBeTruthy()
  })
})

// @sk-test operator-console-v2-1#T2.3: attention deep links and actions (AC-001, AC-002)
describe('Dashboard attention actions', () => {
  it('exposes a deep link for a budget signal', async () => {
    mockBudgets.mockResolvedValue({ data: [{ id: 'b1', tenant_id: 'fintech', spent: 500, hard_limit: 500, type: 'monthly' } as never] })
    renderDash()
    const open = await screen.findByRole('link', { name: /Open/ })
    expect(open.getAttribute('href')).toBe('/budgets')
  })

  it('re-applies a deviated compliance pack for the active workspace', async () => {
    localStorage.setItem('maskchain.workspace', 'acme')
    const { listTenants, getComplianceReport, applyCompliancePack } = await import('../api/tenants')
    const { getShieldCatalog } = await import('../api/shield')
    vi.mocked(listTenants).mockResolvedValue([{ slug: 'acme', name: 'Acme' } as never])
    vi.mocked(getShieldCatalog).mockResolvedValue({ detectors: [], reactions: [], packs: [{ key: 'GDPR', name: 'GDPR' }] })
    vi.mocked(getComplianceReport).mockResolvedValue({ pack_key: 'GDPR', rules: [{ detector_type: 'email', expected_reaction: 'mask', masking: true, status: 'deviated' }] })
    vi.mocked(applyCompliancePack).mockResolvedValue({ pack_key: 'GDPR', rules: [] })

    renderDash()
    fireEvent.click(await screen.findByRole('button', { name: 'Re-apply GDPR' }))
    await waitFor(() => expect(applyCompliancePack).toHaveBeenCalledWith('acme', 'GDPR'))
    localStorage.clear()
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

    const { container } = renderDash()
    await screen.findByText(/Traffic trend/)

    const paths = Array.from(container.querySelectorAll('.spark-svg path')).map((p) => p.getAttribute('d'))
    expect(paths.length).toBe(3)
    expect(new Set(paths).size).toBeGreaterThan(1)
  })
})

// @sk-test operations-hq-charts#T4.3: card labels reflect range and scope (AC-002)
describe('Dashboard labels', () => {
  it('labels cards with the selected range and all-tenants scope', async () => {
    renderDash()
    expect(await screen.findByText('Spend · 7d')).toBeTruthy()
    expect(screen.getByText('Pass rate · last 100 · all tenants')).toBeTruthy()
  })

  it('includes the workspace in labels and scopes pass rate', async () => {
    localStorage.setItem('maskchain.workspace', 'acme')
    renderDash()

    expect(await screen.findByText('Spend · 7d · acme')).toBeTruthy()
    await waitFor(() => expect(mockConversations).toHaveBeenCalledWith(1, 100, { tenant_id: 'acme' }))
    localStorage.clear()
  })
})

// @sk-test operations-hq-charts#T4.3: trend header states metric, range and total (AC-004)
describe('Dashboard trend header', () => {
  it('names the metric and range and updates on switch', async () => {
    renderDash()
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

    renderDash()
    // requests: (100-80)/80 = +25% (distinct from the spend delta)
    expect(await screen.findByText(/▲ 25%/)).toBeTruthy()
  })
})

// @sk-test operations-hq-charts#T4.3: semantics and empty states are explicit (AC-007, AC-008)
describe('Dashboard semantics and empty states', () => {
  it('states the units and meaning of the values', async () => {
    renderDash()
    expect(await screen.findByText(/Tokens = input \+ output/)).toBeTruthy()
  })

  it('shows the chart empty state when the series is empty', async () => {
    mockTokens.mockResolvedValue({ records: [], totals: { total_input_tokens: 10, total_output_tokens: 5 } } as never)
    mockSeries.mockResolvedValue({ series: [] })
    renderDash()
    expect(await screen.findByText('No data for this period')).toBeTruthy()
  })

  it('shows a neutral state when the series is all zero', async () => {
    mockTokens.mockResolvedValue({ records: [], totals: { total_input_tokens: 10, total_output_tokens: 5 } } as never)
    mockSeries.mockResolvedValue({ series: [{ bucket: '2026-08-30T00:00:00Z', input_tokens: 0, output_tokens: 0, cost: 0, requests: 0 }] })
    renderDash()
    expect(await screen.findByText('No activity in this period')).toBeTruthy()
  })
})

// @sk-test ui-visual-polish#T4.1: shared page shell (AC-004)
describe('Dashboard page shell', () => {
  it('renders the shared page header', async () => {
    renderDash()
    expect(await screen.findByRole('heading', { level: 1, name: 'Overview' })).toBeTruthy()
  })
})

// @sk-test hotfix-dashboard-firstrun: onboarding is a stable keyless-install signal
describe('Dashboard first run', () => {
  it('shows onboarding only when there are no virtual keys', async () => {
    mockKeys.mockResolvedValue({ data: [] })
    renderDash()
    expect(await screen.findByText('Welcome to MaskChain')).toBeTruthy()
  })

  it('shows the dashboard when keys exist but there is no usage', async () => {
    renderDash()
    expect(await screen.findByText(/Traffic trend/)).toBeTruthy()
    expect(screen.queryByText('Welcome to MaskChain')).toBeNull()
  })
})
