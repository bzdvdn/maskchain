// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('clamps percentage to 100', () => {
    const { container } = render(<ProgressBar percentage={150} />)
    const fill = container.querySelector('.mc-progress-fill')
    expect((fill as HTMLElement).style.width).toBe('100%')
  })

  it('auto-resolves tone by threshold', () => {
    const { container, rerender } = render(<ProgressBar percentage={40} />)
    expect(container.querySelector('.mc-progress-fill.accent')).toBeTruthy()

    rerender(<ProgressBar percentage={80} />)
    expect(container.querySelector('.mc-progress-fill.warn')).toBeTruthy()

    rerender(<ProgressBar percentage={95} />)
    expect(container.querySelector('.mc-progress-fill.danger')).toBeTruthy()
  })

  it('exposes progressbar semantics', () => {
    const { container } = render(<ProgressBar percentage={86} ariaLabel="spend" />)
    const bar = container.querySelector('[role="progressbar"]')
    expect(bar?.getAttribute('aria-valuenow')).toBe('86')
    expect(bar?.getAttribute('aria-valuemax')).toBe('100')
  })
})