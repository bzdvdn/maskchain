import { useEffect, useState } from 'react'

interface Provider {
  name: string
  api_type: string
  base_url: string
  status: string
  latency_ms?: number
  last_check?: number
}

interface Rule {
  model: string
  tenants: string[]
  providers: string[]
}

function timeAgo(unix: number | undefined): string {
  if (!unix) return '—'
  const sec = Math.floor((Date.now() / 1000) - unix)
  if (sec < 0) return 'now'
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
}

export function Routing() {
  const [providers, setProviders] = useState<Provider[]>([])
  const [rules, setRules] = useState<Rule[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem('admin_token')
    fetch('/api/v1/routing', {
      headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      credentials: 'include',
    })
      .then((r) => {
        if (!r.ok) throw new Error('fetch failed')
        return r.json()
      })
      .then((body) => {
        const d = body.data ?? body
        setProviders(Array.isArray(d.providers) ? d.providers : [])
        setRules(Array.isArray(d.model_routes) ? d.model_routes : [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  return (
    <div>
      <div className="card">
        <h3>Providers</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Name</th><th>Type</th><th>Base URL</th><th>Status</th><th>Latency</th><th>Last Check</th></tr>
            </thead>
            <tbody>
              {providers.map((p, i) => (
                <tr key={i}>
                  <td>{p.name}</td><td>{p.api_type}</td>
                  <td><code>{p.base_url}</code></td>
                  <td><span className={`badge ${p.status === 'up' ? 'badge-up' : 'badge-down'}`}>{p.status}</span></td>
                  <td>{p.latency_ms != null ? `${p.latency_ms}ms` : '—'}</td>
                  <td>{timeAgo(p.last_check)}</td>
                </tr>
              ))}
              {!loading && providers.length === 0 && <tr><td colSpan={6} className="text-muted" style={{ padding: 12 }}>No providers configured</td></tr>}
              {loading && <tr><td colSpan={6} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
      <div className="card">
        <h3>Routing Rules</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Model</th><th>Tenants</th><th>Providers</th></tr>
            </thead>
            <tbody>
              {rules.map((r, i) => (
                <tr key={i}>
                  <td><code>{r.model}</code></td>
                  <td>{r.tenants.join(', ')}</td>
                  <td><code>{r.providers.join(', ')}</code></td>
                </tr>
              ))}
              {!loading && rules.length === 0 && <tr><td colSpan={3} className="text-muted" style={{ padding: 12 }}>No routing rules</td></tr>}
              {loading && <tr><td colSpan={3} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
