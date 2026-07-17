import { useEffect, useState } from 'react'

interface TokenRecord {
  tenant_id: string
  model: string
  total_input_tokens: number
  total_output_tokens: number
  period_start: string
  period_end: string
}

interface CostRecord {
  tenant_id: string
  model: string
  total_cost: number
  request_count: number
  period_start: string
  period_end: string
}

interface TokensData {
  records: TokenRecord[]
  totals: { total_input_tokens: number; total_output_tokens: number }
}

interface CostData {
  records: CostRecord[]
  totals: { total_cost: number; request_count: number }
}

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

export function Analytics() {
  const [tokenRecords, setTokenRecords] = useState<TokenRecord[]>([])
  const [costRecords, setCostRecords] = useState<CostRecord[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/analytics/tokens', { credentials: 'include' }).then((r) => r.json()),
      fetch('/api/v1/analytics/cost', { credentials: 'include' }).then((r) => r.json()),
    ])
      .then(([tokenResp, costResp]) => {
        const t: TokensData = tokenResp.data?.data ?? tokenResp.data ?? tokenResp
        const c: CostData = costResp.data?.data ?? costResp.data ?? costResp
        setTokenRecords(t.records ?? [])
        setCostRecords(c.records ?? [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const totalInput = tokenRecords.reduce((s, r) => s + r.total_input_tokens, 0)
  const totalOutput = tokenRecords.reduce((s, r) => s + r.total_output_tokens, 0)
  const totalTokens = totalInput + totalOutput
  const totalCost = costRecords.reduce((s, r) => s + r.total_cost, 0)
  const totalRequests = costRecords.reduce((s, r) => s + r.request_count, 0)
  const avgTokens = totalRequests > 0 ? Math.round(totalTokens / totalRequests) : 0

  const merged: Record<string, {
    input: number; output: number; cost: number; requests: number; tenants: Set<string>
  }> = {}
  for (const r of tokenRecords) {
    if (!merged[r.model]) merged[r.model] = { input: 0, output: 0, cost: 0, requests: 0, tenants: new Set() }
    merged[r.model].input += r.total_input_tokens
    merged[r.model].output += r.total_output_tokens
    merged[r.model].tenants.add(r.tenant_id)
  }
  for (const r of costRecords) {
    if (!merged[r.model]) merged[r.model] = { input: 0, output: 0, cost: 0, requests: 0, tenants: new Set() }
    merged[r.model].cost += r.total_cost
    merged[r.model].requests += r.request_count
    merged[r.model].tenants.add(r.tenant_id)
  }

  return (
    <div>
      <div className="stats-grid">
        <div className="stat-card"><div className="label">Total Tokens</div><div className="value">{fmtTokens(totalTokens)}</div></div>
        <div className="stat-card"><div className="label">Est. Cost</div><div className="value">${totalCost.toFixed(2)}</div></div>
        <div className="stat-card"><div className="label">Requests</div><div className="value">{totalRequests.toLocaleString()}</div></div>
        <div className="stat-card"><div className="label">Avg Tokens/Req</div><div className="value">{avgTokens}</div></div>
      </div>
      <div className="card">
        <h3>Token Usage by Model</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Model</th><th>Tenants</th><th>Input</th><th>Output</th><th>Total</th><th>Requests</th><th>Cost</th></tr>
            </thead>
            <tbody>
              {Object.entries(merged).map(([model, m]) => (
                <tr key={model}>
                  <td>{model}</td>
                  <td>{m.tenants.size}</td>
                  <td>{fmtTokens(m.input)}</td>
                  <td>{fmtTokens(m.output)}</td>
                  <td>{fmtTokens(m.input + m.output)}</td>
                  <td>{m.requests}</td>
                  <td>${m.cost.toFixed(2)}</td>
                </tr>
              ))}
              {!loading && Object.keys(merged).length === 0 && <tr><td colSpan={7} className="text-muted" style={{ padding: 12 }}>No data</td></tr>}
              {loading && <tr><td colSpan={7} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
