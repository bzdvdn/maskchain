// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useDensity } from './useDensity'

// @sk-test ui-visual-polish#T3.3: density persistence (AC-003)
describe('useDensity', () => {
  beforeEach(() => {
    localStorage.clear()
    delete document.body.dataset.density
  })

  it('defaults to comfortable and persists compact', () => {
    const { result } = renderHook(() => useDensity())
    expect(result.current.density).toBe('comfortable')
    act(() => result.current.toggleDensity())
    expect(result.current.density).toBe('compact')
    expect(document.body.dataset.density).toBe('compact')
    expect(localStorage.getItem('maskchain-density')).toBe('compact')
    act(() => result.current.toggleDensity())
    expect(result.current.density).toBe('comfortable')
  })
})
