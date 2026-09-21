// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Compliance } from './Compliance'

vi.mock('../api/shield', () => ({
  getShieldCatalog: vi.fn().mockResolvedValue({
    detectors: ['regex', 'dictionary', 'prompt_injection'],
    reactions: ['allow', 'block', 'log', 'review'],
    packs: [
      { key: 'HIPAA', name: 'Health Insurance' },
      { key: 'PCI DSS', name: 'Payment Card Industry' },
    ],
  }),
}))

vi.mock('../api/tenants', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/tenants')>()
  return {
    ...actual,
    listTenants: vi.fn().mockResolvedValue([{ slug: 'acme', name: 'Acme' }]),
    getComplianceReport: vi.fn().mockResolvedValue({ pack_key: 'HIPAA', rules: [] }),
    applyCompliancePack: vi.fn(),
  }
})

vi.mock('../components/Toast', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../components/Toast')>()
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useToast: () => ({ toast: vi.fn() }),
  }
})

// @sk-test shield-detector-catalog#T4.3: pack selector is catalog-driven (AC-006)
describe('Compliance catalog', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders pack options from the catalog and nothing hardcoded', async () => {
    render(
      <MemoryRouter>
        <Compliance />
      </MemoryRouter>,
    )

    expect(await screen.findByRole('option', { name: 'Health Insurance' })).toBeTruthy()
    expect(screen.getByRole('option', { name: 'Payment Card Industry' })).toBeTruthy()
    // Not returned by the catalog, so it must not be offered.
    expect(screen.queryByRole('option', { name: 'GDPR' })).toBeNull()
  })
})
