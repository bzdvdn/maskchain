import { useEffect, useState } from 'react'

interface TokenUsage {
  model: string
  tenant: string
  prompt_tokens: number
  completion_tokens: number
  total: number
  cost: number
}

export function Analytics() {
  const [data, setData] = useState<TokenUsage[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/v1/analytics/tokens', { credentials: 'include' })
      .then((r) => {
        if (!r.ok) throw new Error('fetch failed')
        return r.json()
      })
      .then((body) => {
        const d = body.data ?? body
        setData(Array.isArray(d) ? d : d.records ?? [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const totalTokens = Array.isArray(data) ? data.reduce((s, row) => s + (row.total ?? 0), 0) : 0
  const totalCost = Array.isArray(data) ? data.reduce((s, row) => s + (row.cost ?? 0), 0) : 0
  const totalRequests = Array.isArray(data) ? data.length : 0
  const avgTokens = totalRequests > 0 ? Math.round(totalTokens / totalRequests) : 0

  return (
    <div>
      <div className="stats-grid">
        <div className="stat-card"><div className="label">Total Tokens</div><div className="value">{(totalTokens / 1000000).toFixed(1)}M</div></div>
        <div className="stat-card"><div className="label">Est. Cost</div><div className="value">${totalCost.toFixed(2)}</div></div>
        <div className="stat-card"><div className="label">Requests</div><div className="value">{totalRequests.toLocaleString()}</div></div>
        <div className="stat-card"><div className="label">Avg Tokens/Req</div><div className="value">{avgTokens}</div></div>
      </div>
      <div className="card">
        <h3>Token Usage by Model</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Model</th><th>Tenant</th><th>Prompt</th><th>Completion</th><th>Total</th><th>Cost</th></tr>
            </thead>
            <tbody>
              {data.map((row, i) => (
                <tr key={i}>
                  <td>{row.model}</td><td>{row.tenant}</td>
                  <td>{(row.prompt_tokens / 1000).toFixed(0)}K</td>
                  <td>{(row.completion_tokens / 1000).toFixed(0)}K</td>
                  <td>{(row.total / 1000).toFixed(0)}K</td>
                  <td>${row.cost.toFixed(2)}</td>
                </tr>
              ))}
              {!loading && data.length === 0 && <tr><td colSpan={6} className="text-muted" style={{ padding: 12 }}>No data</td></tr>}
              {loading && <tr><td colSpan={6} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
