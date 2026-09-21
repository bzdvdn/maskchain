// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Layout } from './Layout'

vi.mock('../api/tenants', () => ({ listTenants: vi.fn().mockResolvedValue([]) }))
vi.mock('../api/admin', () => ({
  logout: vi.fn(),
  getAdminToken: () => null,
  setAdminToken: vi.fn(),
}))

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
