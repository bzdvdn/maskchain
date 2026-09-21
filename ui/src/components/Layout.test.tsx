// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Layout } from './Layout'
import { getSystemStatus } from '../api/admin'

vi.mock('../api/tenants', () => ({ listTenants: vi.fn().mockResolvedValue([]) }))
vi.mock('../api/admin', () => ({
  logout: vi.fn(),
  getAdminToken: () => null,
  setAdminToken: vi.fn(),
  getSystemStatus: vi.fn(),
}))

const mockStatus = vi.mocked(getSystemStatus)

// @sk-test routing-ia#T4.3: navigation exposes the three routing destinations (AC-008)
describe('Layout navigation', () => {
  it('exposes Providers, Models and Routing as separate links', () => {
    render(
      <MemoryRouter>
        <Layout onLogout={() => {}}>
          <div />
        </Layout>
      </MemoryRouter>,
    )

    expect(screen.getByRole('link', { name: /Providers/ })).toBeTruthy()
    expect(screen.getByRole('link', { name: /^Models$/ })).toBeTruthy()
    expect(screen.getByRole('link', { name: /^Routing$/ })).toBeTruthy()
  })
})

// @sk-test ui-production-readiness#T5.3: live indicator reflects status (AC-010)
describe('Layout live indicator', () => {
  it('shows Healthy when the gateway reports healthy', async () => {
    mockStatus.mockResolvedValue({ health: { status: 'healthy' } } as never)
    render(
      <MemoryRouter>
        <Layout onLogout={() => {}}>
          <div />
        </Layout>
      </MemoryRouter>,
    )
    expect(await screen.findByText('Healthy')).toBeTruthy()
  })

  it('shows Unknown when the status request fails', async () => {
    mockStatus.mockRejectedValue(new Error('down'))
    render(
      <MemoryRouter>
        <Layout onLogout={() => {}}>
          <div />
        </Layout>
      </MemoryRouter>,
    )
    expect(await screen.findByText('Unknown')).toBeTruthy()
  })
})
