// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

function Boom(): React.ReactNode {
  throw new Error('shell exploded')
}

let flakyThrows = true
function Flaky(): React.ReactNode {
  if (flakyThrows) throw new Error('once')
  return <div>recovered</div>
}

// @sk-test ui-production-readiness#T5.3: Shell failures are contained (AC-003)
describe('ErrorBoundary', () => {
  beforeEach(() => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('contains a crash and offers a recover action', () => {
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    )

    expect(screen.getByText('Something went wrong')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy()
  })

  it('recovers after reset', () => {
    flakyThrows = true
    render(
      <ErrorBoundary>
        <Flaky />
      </ErrorBoundary>,
    )

    flakyThrows = false
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(screen.getByText('recovered')).toBeTruthy()
  })
})
