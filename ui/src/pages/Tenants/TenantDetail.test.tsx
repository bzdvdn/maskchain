// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { TenantDetail } from './TenantDetail'

const COMPLIANCE_PACKS = ['HIPAA', 'PCI DSS', 'GDPR', 'Legal', 'SOC 2']

vi.mock('../../api/tenants', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/tenants')>()
  return {
    ...actual,
    getTenant: vi.fn(),
    deleteTenant: vi.fn(),
    applyCompliancePack: vi.fn(),
    getComplianceReport: vi.fn(),
  }
})
vi.mock('../../api/conversations', () => ({
  listConversations: vi.fn().mockResolvedValue({ items: [] as never }),
}))
vi.mock('../../api/shield', () => ({
  getShieldCatalog: vi.fn().mockResolvedValue({
    detectors: ['regex', 'dictionary', 'prompt_injection'],
    reactions: ['allow', 'block', 'log', 'review'],
    packs: ['HIPAA', 'PCI DSS', 'GDPR', 'Legal', 'SOC 2'].map((key) => ({ key, name: key })),
  }),
}))
vi.mock('../../components/Toast', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../components/Toast')>()
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useToast: () => ({ toast: vi.fn() }),
  }
})

import { getTenant, applyCompliancePack, getComplianceReport } from '../../api/tenants'

const mockGet = vi.mocked(getTenant)
const mockApply = vi.mocked(applyCompliancePack)
const mockReport = vi.mocked(getComplianceReport)

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={['/tenants/acme']}>
      <Routes>
        <Route path="/tenants/:slug" element={<TenantDetail />} />
      </Routes>
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockGet.mockResolvedValue({
    slug: 'acme',
    name: 'Acme Corp',
    auth_header: 'X-Tenant',
    retention_mode: 'full',
    created_at: '2026-07-02T10:00:00Z',
    updated_at: '2026-08-01T10:00:00Z',
  } as never)
})

// @sk-test ui-v2-console#T4.5: deviation banner offers report + re-apply (AC-009)
describe('Tenant compliance deviation', () => {
  it('shows the banner for a deviated report and re-applies the pack', async () => {
    mockReport.mockResolvedValue({
      pack_key: 'GDPR',
      rules: [
        { detector_type: 'pii-email', expected_reaction: 'mask', actual_reaction: 'mask', masking: true, status: 'active' },
        { detector_type: 'iban', expected_reaction: 'block', actual_reaction: 'mask', masking: true, status: 'deviated' },
      ],
    })

    renderDetail()

    await screen.findByText('Acme Corp')
    fireEvent.click(screen.getByRole('tab', { name: 'Compliance' }))
    fireEvent.click(screen.getByRole('button', { name: 'GDPR report' }))

    expect(await screen.findByText('Compliance deviation.')).toBeTruthy()
    expect(screen.getByText('Re-apply')).toBeTruthy()
    expect(screen.getAllByText('GDPR').length).toBeGreaterThan(0)

    fireEvent.click(screen.getByRole('button', { name: 'Re-apply' }))

    await waitFor(() => {
      expect(mockApply).toHaveBeenCalledWith('acme', 'GDPR')
    })
  })
})

// @sk-test ui-v2-console#T4.5: no banner when the report is compliant (AC-009)
describe('Tenant compliance clean', () => {
  it('shows no deviation banner for an active-only report', async () => {
    mockReport.mockResolvedValue({
      pack_key: 'GDPR',
      rules: [{ detector_type: 'pii-email', expected_reaction: 'mask', actual_reaction: 'mask', masking: true, status: 'active' }],
    })

    renderDetail()

    await screen.findByText('Acme Corp')
    fireEvent.click(screen.getByRole('tab', { name: 'Compliance' }))
    fireEvent.click(screen.getByRole('button', { name: 'GDPR report' }))

    await waitFor(() => {
      expect(screen.queryByText('Compliance deviation.')).toBeNull()
    })
    // every pack button still available
    for (const p of COMPLIANCE_PACKS) {
      expect(screen.getAllByText(new RegExp(p)).length).toBeGreaterThan(0)
    }
  })
})

// @sk-test hotfix: Policies & Shield tolerates a null pii_config.rules (new tenant)
describe('Tenant policies null rules', () => {
  it('renders the PII shield without crashing when rules is null', async () => {
    mockGet.mockResolvedValue({
      slug: 'acme',
      name: 'Acme Corp',
      auth_header: 'X-Tenant',
      retention_mode: 'full',
      pii_config: { enabled: true, default_action: 'mask', rules: null },
    } as never)

    renderDetail()

    await screen.findByText('Acme Corp')
    fireEvent.click(screen.getByRole('tab', { name: 'Policies & Shield' }))
    expect(await screen.findByText('PII shield')).toBeTruthy()
  })
})