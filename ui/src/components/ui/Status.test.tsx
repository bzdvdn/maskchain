// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatusDot, StatusPill } from './Status'

describe('StatusDot', () => {
  it('applies the tone class and is hidden from accessibility tree', () => {
    const { container } = render(<StatusDot tone="green" />)
    expect(container.querySelector('.mc-dot.green')).toBeTruthy()
    expect(container.querySelector('span[aria-hidden="true"]')).toBeTruthy()
  })
})

describe('StatusPill', () => {
  it('renders tone class and children with an optional dot', () => {
    const { container } = render(<StatusPill tone="red">down</StatusPill>)
    expect(container.querySelector('.mc-pill.red')).toBeTruthy()
    expect(screen.getByText('down')).toBeTruthy()
    expect(container.querySelector('.mc-dot.red')).toBeTruthy()
  })

  it('omits the dot when withDot is false', () => {
    const { container } = render(<StatusPill withDot={false}>enabled</StatusPill>)
    expect(container.querySelector('.mc-dot')).toBeNull()
  })
})