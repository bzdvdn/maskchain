// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AsyncSection } from './AsyncSection'

// @sk-test ui-production-readiness#T5.3: Unified async states (AC-006)
describe('AsyncSection', () => {
  it('shows a skeleton while loading', () => {
    render(<AsyncSection loading error={null}>content</AsyncSection>)
    expect(screen.queryByText('content')).toBeNull()
    expect(screen.getByRole('status')).toBeTruthy()
  })

  it('shows an error with a retry action', () => {
    const onRetry = vi.fn()
    render(
      <AsyncSection loading={false} error={new Error('boom')} onRetry={onRetry}>
        content
      </AsyncSection>,
    )
    expect(screen.getByText('boom')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalled()
  })

  it('shows the empty state when there is no data', () => {
    render(
      <AsyncSection loading={false} error={null} empty emptyMessage="No sessions">
        content
      </AsyncSection>,
    )
    expect(screen.getByText('No sessions')).toBeTruthy()
    expect(screen.queryByText('content')).toBeNull()
  })

  it('renders children when data is present', () => {
    render(
      <AsyncSection loading={false} error={null} empty={false}>
        content
      </AsyncSection>,
    )
    expect(screen.getByText('content')).toBeTruthy()
  })
})
