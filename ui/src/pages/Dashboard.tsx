import { useEffect, useState } from 'react'

interface Metrics {
  rps?: number
  p95?: number
  p99?: number
  errorRate?: number
  activeTenants?: number
  tokensToday?: number
}

interface Session {
  session_id: string
  tenant_id: string
  model: string
  status: string
  token_count: number
  expires_at: string
}

export function Dashboard() {
  const [metrics, setMetrics] = useState<Metrics>({})
  const [sessions, setSessions] = useState<Session[]>([])
  const [error, setError] = useState('')

  function fetchAll() {
    fetch('/api/v1/analytics/tokens', { credentials: 'include' })
      .then((res) => {
        if (!res.ok) throw new Error('Failed to load')
        return res.json()
      })
      .then((data) => {
        const body = data.data ?? data
        setMetrics({
          rps: body.rps ?? body.requests_per_second,
          p95: body.p95 ?? body.latency_p95,
          p99: body.p99 ?? body.latency_p99,
          errorRate: body.error_rate ?? body.errorRate,
          activeTenants: body.active_tenants ?? body.activeTenants,
          tokensToday: body.tokens_today ?? body.tokensToday,
        })
        setError('')
      })
      .catch(() => setError('No data yet'))

    fetch('/api/v1/sessions', { credentials: 'include' })
      .then((r) => {
        if (!r.ok) throw new Error('fetch failed')
        return r.json()
      })
      .then((body) => {
        const d = body.data ?? body
        setSessions(Array.isArray(d.items) ? d.items.slice(0, 5) : Array.isArray(d) ? d.slice(0, 5) : [])
      })
      .catch(() => {})
  }

  useEffect(() => {
    fetchAll()
    const interval = setInterval(fetchAll, 5000)
    return () => clearInterval(interval)
  }, [])

  const cards = [
    { label: 'Requests/sec', value: metrics.rps ?? 0 },
    { label: 'P95 Latency', value: metrics.p95 != null ? `${metrics.p95}ms` : '0ms' },
    { label: 'P99 Latency', value: metrics.p99 != null ? `${metrics.p99}ms` : '0ms' },
    { label: 'Error Rate', value: metrics.errorRate != null ? `${metrics.errorRate}%` : '0%' },
    { label: 'Active Tenants', value: metrics.activeTenants ?? 0 },
    { label: 'Tokens Today', value: metrics.tokensToday != null ? `${(metrics.tokensToday / 1000000).toFixed(1)}M` : '0M' },
  ]

  return (
    <div>
      <div className="stats-grid">
        {cards.map((card) => (
          <div key={card.label} className="stat-card">
            <div className="label">{card.label}</div>
            <div className="value">{card.value}</div>
          </div>
        ))}
      </div>

      {error && <p className="text-muted" style={{ marginBottom: 24 }}>{error}</p>}

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
