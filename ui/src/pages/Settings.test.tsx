// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Settings } from './Settings'

import { getSystemStatus, type SystemStatus } from '../api/admin'

const mockStatus = vi.mocked(getSystemStatus)

function status(over: Partial<SystemStatus> = {}): SystemStatus {
  return {
    version: 'v2.4.1 (commit: abc)',
    uptime_seconds: 90000,
    key_at_rest: { configured: true, cipher: 'AES-256-GCM' },
    health: {
      status: 'ok',
      checks: {
        database: { status: 'ok', latency_ms: 2 },
        valkey: { status: 'up', latency_ms: 1 },
      },
    },
    config_diff: { watched: true, sections: ['routing'] },
    ...over,
  }
}

vi.mock('../api/admin', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/admin')>()
  return {
    ...actual,
    getSystemStatus: vi.fn(),
  }
})

beforeEach(() => {
  vi.clearAllMocks()
})

// @sk-test ui-v2-console#T4.5: settings renders live status without hardcoded literals (AC-010)
describe('Settings live status', () => {
  it('renders version, stores and config diff from the status payload', async () => {
    mockStatus.mockResolvedValue(status())

    render(<Settings />)

    expect(await screen.findByText('v2.4.1 (commit: abc)')).toBeTruthy()
    expect(screen.getByText('configured')).toBeTruthy()
    expect(screen.getByText('AES-256-GCM')).toBeTruthy()
    expect(screen.getByText('database')).toBeTruthy()
    expect(screen.getByText('valkey')).toBeTruthy()
    expect(screen.getByText('routing')).toBeTruthy()
  })

  it('reports unconfigured at-rest encryption', async () => {
    mockStatus.mockResolvedValue(status({ key_at_rest: { configured: false } }))

    render(<Settings />)

    expect(await screen.findByText('not configured')).toBeTruthy()
  })

  it('flags a config diff as needs-attention', async () => {
    mockStatus.mockResolvedValue(status())

    render(<Settings />)

    expect(await screen.findByText(/Live config differs from the file/)).toBeTruthy()
  })
})