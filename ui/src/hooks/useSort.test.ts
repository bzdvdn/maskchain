// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useSort, sortRows } from '../hooks/useSort'

describe('useSort', () => {
  it('toggles direction on same key', () => {
    const { result } = renderHook(() => useSort<{ a: number }>('a', 'asc'))
    expect(result.current.key).toBe('a')
    expect(result.current.dir).toBe('asc')

    act(() => result.current.toggle('a'))
    expect(result.current.dir).toBe('desc')

    act(() => result.current.toggle('a'))
    expect(result.current.dir).toBe('asc')
  })

  it('switches key and resets to asc on new key', () => {
    const { result } = renderHook(() => useSort<{ a: number; b: number }>('a', 'desc'))

    act(() => result.current.toggle('b'))
    expect(result.current.key).toBe('b')
    expect(result.current.dir).toBe('asc')
  })
})

describe('sortRows', () => {
  const rows = [
    { id: 'b', n: 2 },
    { id: 'a', n: 3 },
    { id: 'c', n: 1 },
  ]

  it('sorts ascending by default', () => {
    expect(sortRows(rows, 'id', 'asc').map((r) => r.id)).toEqual(['a', 'b', 'c'])
  })

  it('sorts descending', () => {
    expect(sortRows(rows, 'n', 'desc').map((r) => r.n)).toEqual([3, 2, 1])
  })

  it('returns original order when no key', () => {
    expect(sortRows(rows, null, 'asc')).toBe(rows)
  })
})
