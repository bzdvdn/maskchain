import { describe, it, expect } from 'vitest'
import { absDate, relativeTime, groupNum, money, fmtTokens } from './format'

describe('absDate', () => {
  it('formats an ISO timestamp as locale-neutral local datetime', () => {
    const d = new Date(2026, 7, 30, 14, 5)
    expect(absDate(d.toISOString())).toBe('2026-08-30 14:05')
  })

  it('returns a dash for invalid input', () => {
    expect(absDate('nope')).toBe('—')
  })
})

describe('relativeTime', () => {
  const now = new Date('2026-08-30T12:00:00Z')

  it('returns just now under a minute', () => {
    expect(relativeTime('2026-08-30T12:00:20Z', now)).toBe('just now')
    expect(relativeTime('2026-08-30T11:59:50Z', now)).toBe('just now')
  })

  it('scales minutes, hours and days', () => {
    expect(relativeTime('2026-08-30T11:45:00Z', now)).toBe('15m ago')
    expect(relativeTime('2026-08-30T09:00:00Z', now)).toBe('3h ago')
    expect(relativeTime('2026-08-28T12:00:00Z', now)).toBe('2d ago')
  })

  it('handles future timestamps without a locale string', () => {
    expect(relativeTime('2026-08-30T12:30:00Z', now)).toBe('in 30m')
  })

  it('falls back to absolute date beyond a week', () => {
    expect(relativeTime('2026-08-01T12:00:00Z', now)).toBe(absDate('2026-08-01T12:00:00Z'))
  })

  it('returns a dash for invalid input', () => {
    expect(relativeTime('nope', now)).toBe('—')
  })
})

describe('money', () => {
  it('formats USD with thousands grouping', () => {
    expect(money(1284.3)).toBe('$1,284.30')
  })

  it('uses an explicit currency prefix for non-USD', () => {
    expect(money(94.5, 'EUR')).toBe('EUR 94.50')
  })

  it('returns a dash for non-finite input', () => {
    expect(money(Number.NaN)).toBe('—')
  })
})

describe('fmtTokens', () => {
  it('scales thousands and millions', () => {
    expect(fmtTokens(147)).toBe('147')
    expect(fmtTokens(1200)).toBe('1.2K')
    expect(fmtTokens(12_400_000)).toBe('12.4M')
  })
})

describe('groupNum', () => {
  it('groups digits without locale dependence', () => {
    expect(groupNum(84209)).toBe('84,209')
    expect(groupNum('1000000')).toBe('1,000,000')
  })
})