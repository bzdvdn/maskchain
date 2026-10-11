// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { DictionaryEditor, entriesToLines, linesToEntries } from './DictionaryEditor'
import type { DictionaryItem } from '../api/tenants'

// @sk-test ui-dictionary-editor#T3.1: structured entries round-trip (AC-003)
describe('entries (de)serialization', () => {
  it('round-trips structured entries without flattening them', () => {
    const entries = [{ name: 'John', email: 'j@c' }, 'plain', 42]
    expect(linesToEntries(entriesToLines(entries))).toEqual(entries)
  })

  it('keeps non-JSON lines as strings and drops blank lines', () => {
    expect(linesToEntries('hello\n  \nworld')).toEqual(['hello', 'world'])
  })
})

// @sk-test ui-dictionary-editor#T3.1: add/remove + errors (AC-005)
describe('DictionaryEditor', () => {
  it('adds a dictionary with an exact default match mode', () => {
    const onChange = vi.fn()
    render(<DictionaryEditor value={[]} onChange={onChange} />)
    fireEvent.click(screen.getByText('+ Add dictionary'))
    expect(onChange).toHaveBeenCalledWith([{ name: '', entries: [], match_mode: 'exact' }])
  })

  it('removes the dictionary at the clicked index', () => {
    const onChange = vi.fn()
    const value: DictionaryItem[] = [
      { name: 'a', entries: ['x'], match_mode: 'exact' },
      { name: 'b', entries: ['y'], match_mode: 'contains' },
    ]
    render(<DictionaryEditor value={value} onChange={onChange} />)
    fireEvent.click(screen.getByLabelText('Remove dictionary 1'))
    expect(onChange).toHaveBeenCalledWith([value[1]])
  })

  it('shows a per-dictionary error', () => {
    render(
      <DictionaryEditor
        value={[{ name: '', entries: [], match_mode: 'exact' }]}
        onChange={() => {}}
        errors={{ 0: 'Name is required' }}
      />,
    )
    expect(screen.getByText('Name is required')).toBeTruthy()
  })
})
