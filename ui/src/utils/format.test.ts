import { describe, it, expect } from 'vitest'
import { absDate, relativeTime, groupNum, money, fmtTokens, rangeLabel, chartTickLabel, chartTooltipLabel, rangeSpanMs } from './format'

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
describe('rangeLabel', () => {
  it('maps presets to their names', () => {
    expect(rangeLabel('today')).toBe('Today')
    expect(rangeLabel('yesterday')).toBe('Yesterday')
    expect(rangeLabel('7d')).toBe('7d')
    expect(rangeLabel('30d')).toBe('30d')
    expect(rangeLabel('all')).toBe('All')
  })

  it('renders a custom range as a deterministic date span', () => {
    expect(rangeLabel('custom', '2026-07-01T00:00:00Z', '2026-07-07T00:00:00Z')).toBe('Jul 1 – Jul 7')
  })

  it('falls back to Custom when bounds are missing or invalid', () => {
    expect(rangeLabel('custom')).toBe('Custom')
    expect(rangeLabel('custom', 'not-a-date', 'also-bad')).toBe('Custom')
  })
})

describe('chartTickLabel', () => {
  const iso = new Date(2026, 8, 21, 14, 5).toISOString()
  const HOUR = 3600_000
  const DAY = 86_400_000

  it('shows clock time for short spans', () => {
    expect(chartTickLabel(iso, 6 * HOUR)).toBe('14:05')
    expect(chartTickLabel(iso, 48 * HOUR)).toBe('14:05')
  })

  it('shows date and time up to a week', () => {
    expect(chartTickLabel(iso, 7 * DAY)).toBe('21.09 14:05')
  })

  it('shows date only beyond a week (no fake 00:00)', () => {
    expect(chartTickLabel(iso, 30 * DAY)).toBe('21.09')
    expect(chartTickLabel(iso, Number.POSITIVE_INFINITY)).toBe('21.09')
  })

  it('returns a dash for invalid input', () => {
    expect(chartTickLabel('not-a-date', 30 * DAY)).toBe('—')
  })
})

describe('chartTooltipLabel', () => {
  it('always shows the full date and time', () => {
    const iso = new Date(2026, 8, 21, 14, 5).toISOString()
    expect(chartTooltipLabel(iso)).toBe('21.09.2026 14:05')
  })

  it('returns a dash for invalid input', () => {
    expect(chartTooltipLabel('nope')).toBe('—')
  })
})

describe('rangeSpanMs', () => {
  const DAY = 86_400_000

  it('maps presets to their window', () => {
    expect(rangeSpanMs('today')).toBe(DAY)
    expect(rangeSpanMs('yesterday')).toBe(2 * DAY)
    expect(rangeSpanMs('7d')).toBe(7 * DAY)
    expect(rangeSpanMs('30d')).toBe(30 * DAY)
  })

  it('treats all as unbounded', () => {
    expect(rangeSpanMs('all')).toBe(Number.POSITIVE_INFINITY)
  })

  it('uses custom bounds and falls back to zero', () => {
    expect(rangeSpanMs('custom', '2026-07-01T00:00:00Z', '2026-07-07T00:00:00Z')).toBe(6 * DAY)
    expect(rangeSpanMs('custom')).toBe(0)
    expect(rangeSpanMs('custom', 'bad', 'worse')).toBe(0)
  })
})
