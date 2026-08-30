// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ChipInput } from './ChipTag'

describe('ChipInput', () => {
  it('adds a chip on Enter', () => {
    const onChange = vi.fn()
    render(<ChipInput value={[]} onChange={onChange} placeholder="model" />)

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'gpt-4o' } })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' })
    expect(onChange).toHaveBeenCalledWith(['gpt-4o'])
  })

  it('commits on comma', () => {
    const onChange = vi.fn()
    render(<ChipInput value={['a']} onChange={onChange} />)

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'b' } })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: ',' })
    expect(onChange).toHaveBeenCalledWith(['a', 'b'])
  })

  it('ignores duplicates', () => {
    const onChange = vi.fn()
    render(<ChipInput value={['a']} onChange={onChange} />)

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'a' } })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('honours the max limit', () => {
    const onChange = vi.fn()
    render(<ChipInput value={['a', 'b']} onChange={onChange} max={2} />)

    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'c' } })
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('removes the last chip on backspace with an empty draft', () => {
    const onChange = vi.fn()
    render(<ChipInput value={['a', 'b']} onChange={onChange} />)

    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Backspace' })
    expect(onChange).toHaveBeenCalledWith(['a'])
  })

  it('removes a chip via its remove button', () => {
    const onChange = vi.fn()
    render(<ChipInput value={['a', 'b']} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Remove a' }))
    expect(onChange).toHaveBeenCalledWith(['b'])
  })
})