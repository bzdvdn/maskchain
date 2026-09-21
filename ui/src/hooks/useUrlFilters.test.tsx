// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, useLocation } from 'react-router-dom'
import { useUrlFilters } from './useUrlFilters'

function Harness() {
  const url = useUrlFilters()
  const location = useLocation()
  return (
    <div>
      <span data-testid="tenant">{url.get('tenant', 'none')}</span>
      <span data-testid="page">{url.getInt('page', 1)}</span>
      <span data-testid="search">{location.search}</span>
      <button onClick={() => url.set({ tenant: 'acme', page: 2 })}>apply</button>
      <button onClick={() => url.set({ tenant: undefined })}>clear</button>
    </div>
  )
}

// @sk-test ui-production-readiness#T5.3: URL round-trip of filters (AC-009)
describe('useUrlFilters', () => {
  it('writes filters to the URL and reads them back', () => {
    render(
      <MemoryRouter initialEntries={['/conversations']}>
        <Harness />
      </MemoryRouter>,
    )

    fireEvent.click(screen.getByText('apply'))
    expect(screen.getByTestId('tenant').textContent).toBe('acme')
    expect(screen.getByTestId('page').textContent).toBe('2')
    expect(screen.getByTestId('search').textContent).toContain('tenant=acme')

    fireEvent.click(screen.getByText('clear'))
    expect(screen.getByTestId('tenant').textContent).toBe('none')
  })

  it('hydrates from an existing URL', () => {
    render(
      <MemoryRouter initialEntries={['/conversations?tenant=beta&page=3']}>
        <Harness />
      </MemoryRouter>,
    )
    expect(screen.getByTestId('tenant').textContent).toBe('beta')
    expect(screen.getByTestId('page').textContent).toBe('3')
  })
})
