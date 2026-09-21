// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TimeSeriesChart } from './TimeSeriesChart'

const points = [
  { bucket: '2026-08-30T00:00:00Z', input_tokens: 100, output_tokens: 50, cost: 1.2, requests: 30 },
  { bucket: '2026-08-31T00:00:00Z', input_tokens: 200, output_tokens: 80, cost: 2.4, requests: 45 },
]

// @sk-test operations-hq-charts#T4.2: tokens split legend + accessible name (AC-003, AC-007)
describe('TimeSeriesChart tokens (default)', () => {
  it('renders the Input/Output legend and an accessible name', () => {
    render(<TimeSeriesChart data={points} />)
    expect(screen.getByText('Input')).toBeTruthy()
    expect(screen.getByText('Output')).toBeTruthy()
    expect(screen.getByRole('img', { name: /Tokens trend over 2 buckets/ })).toBeTruthy()
  })
})

// @sk-test operations-hq-charts#T4.2: single-series metrics render one legend entry (AC-003)
describe('TimeSeriesChart single-series metrics', () => {
  it('renders a Cost legend for the cost metric', () => {
    render(<TimeSeriesChart data={points} metric="cost" />)
    expect(screen.getByText('Cost')).toBeTruthy()
    expect(screen.getByRole('img', { name: /Cost trend/ })).toBeTruthy()
  })

  it('renders a Requests legend for the requests metric', () => {
    render(<TimeSeriesChart data={points} metric="requests" />)
    expect(screen.getByText('Requests')).toBeTruthy()
    expect(screen.getByRole('img', { name: /Requests trend/ })).toBeTruthy()
  })
})

// @sk-test operations-hq-charts#T4.2: empty and all-zero series are explicit (AC-008)
describe('TimeSeriesChart empty states', () => {
  it('shows an empty message for no data', () => {
    render(<TimeSeriesChart data={[]} />)
    expect(screen.getByText('No data for this period')).toBeTruthy()
    expect(screen.getByRole('img', { name: /no data for this period/ })).toBeTruthy()
  })

  it('shows a neutral state for all-zero values', () => {
    render(<TimeSeriesChart data={[{ bucket: '2026-08-30T00:00:00Z', input_tokens: 0, output_tokens: 0 }]} />)
    expect(screen.getByText('No activity in this period')).toBeTruthy()
  })

  it('notes a single data point', () => {
    render(<TimeSeriesChart data={[points[0]]} />)
    expect(screen.getByText(/Single data point/)).toBeTruthy()
  })
})

// @sk-test chart-time-formatting#T3.3: dense hourly data and explicit span render (AC-005)
describe('TimeSeriesChart span and density', () => {
  it('renders 168 hourly points with a week span without error', () => {
    const start = new Date(2026, 8, 15, 0, 0).getTime()
    const dense = Array.from({ length: 168 }, (_, i) => ({
      bucket: new Date(start + i * 3600_000).toISOString(),
      input_tokens: 100 + i,
      output_tokens: 50,
      cost: 0.1,
      requests: 1,
    }))

    render(<TimeSeriesChart data={dense} spanMs={7 * 86_400_000} />)

    expect(screen.getByText('Input')).toBeTruthy()
    expect(screen.getByRole('img', { name: /Tokens trend over 168 buckets/ })).toBeTruthy()
  })
})
