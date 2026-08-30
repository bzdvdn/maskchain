// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Segmented } from './Segmented'

describe('Segmented', () => {
  it('marks the active option and reports changes', () => {
    const onChange = vi.fn()
    render(
      <Segmented
        ariaLabel="metric"
        options={[
          { key: 'tokens', label: 'Tokens' },
          { key: 'cost', label: 'Cost' },
          { key: 'requests', label: 'Requests' },
        ]}
        value="tokens"
        onChange={onChange}
      />,
    )

    const cost = screen.getByRole('button', { name: 'Cost' })
    fireEvent.click(cost)
    expect(onChange).toHaveBeenCalledWith('cost')
    expect(screen.getByRole('button', { name: 'Tokens' }).getAttribute('aria-pressed')).toBe('true')
  })
})