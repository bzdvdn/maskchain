// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Layout } from './Layout'
import { AttentionProvider } from './AttentionProvider'

vi.mock('../api/admin', () => ({ getSystemStatus: vi.fn(), logout: vi.fn() }))
vi.mock('../api/tenants', () => ({
  listTenants: vi.fn().mockResolvedValue([]),
  getComplianceReport: vi.fn(),
  applyCompliancePack: vi.fn(),
}))
vi.mock('../api/shield', () => ({ getShieldCatalog: vi.fn().mockResolvedValue({ detectors: [], reactions: [], packs: [] }) }))
vi.mock('../api/budgets', () => ({ listBudgets: vi.fn(), listBudgetsByTenant: vi.fn() }))
vi.mock('../api/routing', () => ({ listProviders: vi.fn() }))
vi.mock('../api/keys', () => ({ listKeys: vi.fn() }))
vi.mock('../api/conversations', () => ({ listConversations: vi.fn() }))

import { getSystemStatus } from '../api/admin'
import { listBudgets } from '../api/budgets'
import { listProviders } from '../api/routing'
import { listKeys } from '../api/keys'
import { listConversations } from '../api/conversations'

function renderLayout() {
  return render(
    <MemoryRouter>
      <AttentionProvider>
        <Layout onLogout={() => {}}>
          <div>content</div>
        </Layout>
      </AttentionProvider>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  vi.mocked(getSystemStatus).mockResolvedValue({ health: { status: 'healthy' } } as never)
  vi.mocked(listProviders).mockResolvedValue([])
  vi.mocked(listKeys).mockResolvedValue({ data: [] })
  vi.mocked(listConversations).mockResolvedValue({ items: [] } as never)
})

// @sk-test operator-console-v2-1#T2.3: header critical badge (AC-004)
describe('Layout attention badge', () => {
  it('hides the badge when there are no critical items', async () => {
    vi.mocked(listBudgets).mockResolvedValue({ data: [] })
    renderLayout()
    await waitFor(() => expect(screen.queryByLabelText(/critical items/)).toBeNull())
  })

  it('shows the un-acked critical count', async () => {
    vi.mocked(listBudgets).mockResolvedValue({ data: [{ id: 'b1', tenant_id: 'fintech', spent: 500, hard_limit: 500, type: 'monthly' } as never] })
    renderLayout()
    expect(await screen.findByLabelText('1 critical items')).toBeTruthy()
  })
})
