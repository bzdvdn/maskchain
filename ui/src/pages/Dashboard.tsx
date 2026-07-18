import { useEffect, useState, useCallback } from 'react'
import { TimeRangePicker, useRange, defaultRange, type RangeValue } from '../components/TimeRangePicker'
import { TimeSeriesChart } from '../components/TimeSeriesChart'

interface TokenRecord {
  tenant_id: string
  model: string
  total_input_tokens: number
  total_output_tokens: number
}

interface TokenTotals {
  total_input_tokens: number
  total_output_tokens: number
}

interface TokensData {
  records: TokenRecord[]
  totals: TokenTotals
}

interface Session {
  session_id: string
  tenant_id: string
  model: string
  status: string
  token_count: number
  expires_at: string
}

interface SessionsData {
  items: Session[]
}

interface SeriesPoint {
  bucket: string
  input_tokens: number
  output_tokens: number
}

export function Dashboard() {
  const [range, setRange] = useState<RangeValue>(defaultRange)
  const { from, to } = useRange(range)
  const [records, setRecords] = useState<TokenRecord[]>([])
  const [totals, setTotals] = useState<TokenTotals>({ total_input_tokens: 0, total_output_tokens: 0 })
  const [series, setSeries] = useState<SeriesPoint[]>([])
  const [sessions, setSessions] = useState<Session[]>([])
  const [error, setError] = useState('')
  const [refreshSec, setRefreshSec] = useState(10)

  const fetchTokens = useCallback(() => {
    fetch(`/api/v1/analytics/tokens?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { credentials: 'include' })
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load')
        return res.json()
      })
      .then((envelope) => {
        const body: TokensData = envelope.data?.data ?? envelope.data ?? envelope
        setRecords(body.records ?? [])
        setTotals(body.totals ?? { total_input_tokens: 0, total_output_tokens: 0 })
        setError('')
      })
      .catch(() => setError('No data yet'))
  }, [from, to])

  const fetchSeries = useCallback(() => {
    fetch(`/api/v1/analytics/timeseries?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`, { credentials: 'include' })
      .then((r) => r.json())
      .then((envelope) => {
        const d = envelope.data?.data ?? envelope.data ?? envelope
        setSeries(Array.isArray(d.series) ? d.series : [])
      })
      .catch(() => {})
  }, [from, to])

  function fetchSessions() {
    fetch('/api/v1/sessions', { credentials: 'include' })
      .then((r) => {
        if (!r.ok) throw new Error('fetch failed')
        return r.json()
      })
      .then((body) => {
        const d: SessionsData = body.data ?? body
        setSessions(Array.isArray(d.items) ? d.items.slice(0, 5) : [])
      })
      .catch(() => {})
  }

  useEffect(() => {
    fetchTokens()
    fetchSeries()
    fetchSessions()
    if (refreshSec > 0) {
      const interval = setInterval(() => {
        fetchTokens()
        fetchSeries()
        fetchSessions()
      }, refreshSec * 1000)
      return () => clearInterval(interval)
    }
  }, [fetchTokens, fetchSeries, refreshSec])

  const tokensTotal = totals.total_input_tokens + totals.total_output_tokens
  const activeTenants = new Set(records.map((r) => r.tenant_id)).size

  const cards = [
    { label: 'Requests', value: records.length },
    { label: 'Input Tokens', value: totals.total_input_tokens.toLocaleString() },
    { label: 'Output Tokens', value: totals.total_output_tokens.toLocaleString() },
    { label: 'Total Tokens', value: tokensTotal.toLocaleString() },
    { label: 'Active Tenants', value: activeTenants },
    { label: 'Models Used', value: new Set(records.map((r) => r.model)).size },
  ]

  const modelTotals: Record<string, { input: number; output: number }> = {}
  for (const r of records) {
    if (!modelTotals[r.model]) modelTotals[r.model] = { input: 0, output: 0 }
    modelTotals[r.model].input += r.total_input_tokens
    modelTotals[r.model].output += r.total_output_tokens
  }

  return (
    <div>
      <div className="card" style={{ marginBottom: 24 }}>
        <div className="card-header-row">
          <h3>Token Usage Trend <span className="text-muted" style={{ fontSize: 12, fontWeight: 400 }}>{`${series.length} pts`}</span></h3>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
            <span className="text-muted" style={{ fontSize: 12 }}>Refresh:</span>
            {[0, 5, 10, 30].map((s) => (
              <button
                key={s}
                className="btn btn-small"
                onClick={() => setRefreshSec(s)}
                style={refreshSec === s ? { borderColor: 'var(--accent)', color: 'var(--accent)' } : {}}
              >
                {s === 0 ? 'Off' : `${s}s`}
              </button>
            ))}
            <TimeRangePicker value={range} onChange={setRange} />
          </div>
        </div>
        <TimeSeriesChart data={series} />
      </div>

      <div className="stats-grid">
        {cards.map((card) => (
          <div key={card.label} className="stat-card">
            <div className="label">{card.label}</div>
            <div className="value">{card.value}</div>
          </div>
        ))}
      </div>

      {error && <p className="text-muted" style={{ marginBottom: 24 }}>{error}</p>}

      {Object.keys(modelTotals).length > 0 && (
        <div className="card" style={{ marginBottom: 24 }}>
          <h3>Token Usage by Model</h3>
          <div className="table-wrap">
            <table>
              <thead>
                <tr><th>Model</th><th>Input Tokens</th><th>Output Tokens</th><th>Total</th></tr>
              </thead>
              <tbody>
                {Object.entries(modelTotals).map(([model, counts]) => (
                  <tr key={model}>
                    <td><code>{model}</code></td>
                    <td>{counts.input.toLocaleString()}</td>
                    <td>{counts.output.toLocaleString()}</td>
                    <td>{(counts.input + counts.output).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <div className="card">
        <h3>Active Sessions</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Session ID</th><th>Tenant</th><th>Model</th><th>Status</th><th>TTL</th></tr>
            </thead>
            <tbody>
              {sessions.map((s) => (
                <tr key={s.session_id}>
                  <td><code>{s.session_id.slice(0, 12)}...</code></td>
                  <td>{s.tenant_id}</td>
                  <td>{s.model}</td>
                  <td><span className={`badge ${s.status === 'active' ? 'badge-up' : s.status === 'expired' ? 'badge-down' : 'badge-warn'}`}>{s.status}</span></td>
                  <td>{s.expires_at ? Math.round((new Date(s.expires_at).getTime() - Date.now()) / 60000) + 'm' : '—'}</td>
                </tr>
              ))}
              {sessions.length === 0 && <tr><td colSpan={5} className="text-muted" style={{ padding: 12 }}>No active sessions</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
