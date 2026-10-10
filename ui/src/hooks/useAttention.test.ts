// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { isSnoozed, useAttention, computeAttention } from './useAttention'

vi.mock('../api/budgets', () => ({ listBudgets: vi.fn(), listBudgetsByTenant: vi.fn() }))
vi.mock('../api/routing', () => ({ listProviders: vi.fn() }))
vi.mock('../api/keys', () => ({ listKeys: vi.fn() }))
vi.mock('../api/conversations', () => ({ listConversations: vi.fn() }))
vi.mock('../api/tenants', () => ({ listTenants: vi.fn(), getComplianceReport: vi.fn(), applyCompliancePack: vi.fn() }))
vi.mock('../api/shield', () => ({ getShieldCatalog: vi.fn() }))

import { listBudgets } from '../api/budgets'
import { listProviders } from '../api/routing'
import { listKeys } from '../api/keys'
import { listConversations } from '../api/conversations'
import { listTenants, getComplianceReport, applyCompliancePack } from '../api/tenants'
import { getShieldCatalog } from '../api/shield'

const mBudgets = vi.mocked(listBudgets)
const mProviders = vi.mocked(listProviders)
const mKeys = vi.mocked(listKeys)
const mConv = vi.mocked(listConversations)
const mTenants = vi.mocked(listTenants)
const mReport = vi.mocked(getComplianceReport)
const mApply = vi.mocked(applyCompliancePack)
const mCatalog = vi.mocked(getShieldCatalog)

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  mBudgets.mockResolvedValue({ data: [] })
  mProviders.mockResolvedValue([])
  mKeys.mockResolvedValue({ data: [] })
  mConv.mockResolvedValue({ items: [] } as never)
  mTenants.mockResolvedValue([])
  mCatalog.mockResolvedValue({ detectors: [], reactions: [], packs: [] })
})

// @sk-test operator-console-v2-1#T1.3: snooze predicate is fail-open (AC-003)
describe('isSnoozed', () => {
  it('is true only for a future finite timestamp', () => {
    expect(isSnoozed({ k: 2000 }, 'k', 1000)).toBe(true)
    expect(isSnoozed({ k: 500 }, 'k', 1000)).toBe(false)
    expect(isSnoozed({ k: Number.NaN }, 'k', 1000)).toBe(false)
    expect(isSnoozed({}, 'k', 1000)).toBe(false)
  })
})

// @sk-test operator-console-v2-1#T1.3: acknowledge hides and persists (AC-003, AC-004)
describe('useAttention ack', () => {
  it('hides an acknowledged item and persists it', async () => {
    mBudgets.mockResolvedValue({ data: [{ id: 'b1', tenant_id: 'fintech', spent: 500, hard_limit: 500, type: 'monthly' } as never] })
    const { result } = renderHook(() => useAttention(''))
    await act(async () => {
      await result.current.reload()
    })
    expect(result.current.items).toHaveLength(1)
    expect(result.current.criticalCount).toBe(1)

    act(() => result.current.ack('budget:b1'))
    expect(result.current.items).toHaveLength(0)
    expect(result.current.criticalCount).toBe(0)
    expect(JSON.parse(localStorage.getItem('maskchain.attention.acked') ?? '[]')).toContain('budget:b1')
  })

  it('reappears after the snooze expires', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000_000)
    mBudgets.mockResolvedValue({ data: [{ id: 'b1', tenant_id: 'fintech', spent: 500, hard_limit: 500, type: 'monthly' } as never] })
    const { result, rerender } = renderHook(() => useAttention(''))
    await act(async () => {
      await result.current.reload()
    })
    act(() => result.current.snooze('budget:b1', 5000))
    expect(result.current.items).toHaveLength(0)

    vi.setSystemTime(1_010_000)
    rerender()
    expect(result.current.items).toHaveLength(1)
    vi.useRealTimers()
  })
})

// @sk-test operator-console-v2-1#T1.3: compliance source is bounded to the workspace (AC-002)
describe('computeAttention compliance', () => {
  it('skips compliance when no workspace is selected', async () => {
    await computeAttention('')
    expect(mCatalog).not.toHaveBeenCalled()
  })

  it('produces a re-applyable deviation item for the workspace tenant', async () => {
    mTenants.mockResolvedValue([{ slug: 'acme', name: 'Acme' } as never])
    mCatalog.mockResolvedValue({ detectors: [], reactions: [], packs: [{ key: 'GDPR', name: 'GDPR' }] })
    mReport.mockResolvedValue({ pack_key: 'GDPR', rules: [{ detector_type: 'email', expected_reaction: 'mask', masking: true, status: 'deviated' }] })
    mApply.mockResolvedValue({ pack_key: 'GDPR', rules: [] })

    const items = await computeAttention('acme')
    const item = items.find((i) => i.key === 'compliance:acme:GDPR')
    expect(item?.actionLabel).toBe('Re-apply GDPR')
    await item?.action?.()
    expect(mApply).toHaveBeenCalledWith('acme', 'GDPR')
  })
})
