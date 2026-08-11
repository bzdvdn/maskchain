import { useEffect, useMemo, useState } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { TimeRangePicker, useRange, type RangeValue } from '../components/TimeRangePicker'
import { TimeSeriesChart } from '../components/TimeSeriesChart'
import { EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { useSort, sortRows } from '../hooks/useSort'
import { listConversations } from '../api/conversations'
import {
  getAnalyticsCost,
  getAnalyticsSeries,
  getAnalyticsTokens,
  type CostRecord,
  type SeriesPoint,
  type TokenRecord,
} from '../api/analytics'

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

export function Analytics() {
  const [range, setRange] = useState<RangeValue>({ mode: '30d', from: '', to: '' })
  const { from, to } = useRange(range)
  const [tokenRecords, setTokenRecords] = useState<TokenRecord[]>([])
  const [costRecords, setCostRecords] = useState<CostRecord[]>([])
  const [series, setSeries] = useState<SeriesPoint[]>([])
  const [loading, setLoading] = useState(true)
  const [rateByModel, setRateByModel] = useState<Record<string, { ok: number; error: number; blocked: number }>>({})

  useEffect(() => {
    setLoading(true)
    Promise.all([
      getAnalyticsTokens(from, to),
      getAnalyticsCost(from, to),
      getAnalyticsSeries(from, to),
    ])
      .then(([t, c, s]) => {
        setTokenRecords(t.records ?? [])
        setCostRecords(c.records ?? [])
        setSeries(Array.isArray(s.series) ? s.series : [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [from, to])

  useEffect(() => {
    listConversations(1, 100)
      .then((res) => {
        const agg: Record<string, { ok: number; error: number; blocked: number }> = {}
        for (const c of res.items) {
          const m = c.model || 'unknown'
          if (!agg[m]) agg[m] = { ok: 0, error: 0, blocked: 0 }
          if (c.status === 'ok') agg[m].ok++
          else if (c.status === 'blocked') agg[m].blocked++
          else agg[m].error++
        }
        setRateByModel(agg)
      })
      .catch(() => {})
  }, [])

  const totalInput = tokenRecords.reduce((s, r) => s + r.total_input_tokens, 0)
  const totalOutput = tokenRecords.reduce((s, r) => s + r.total_output_tokens, 0)
  const totalTokens = totalInput + totalOutput
  const totalCost = costRecords.reduce((s, r) => s + r.total_cost, 0)
  const totalRequests = costRecords.reduce((s, r) => s + r.request_count, 0)
  const avgTokens = totalRequests > 0 ? Math.round(totalTokens / totalRequests) : 0

  function exportCSV(kind: 'tokens' | 'cost' | 'timeseries') {
    const qs = `?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&format=csv`
    window.open(`/api/v1/analytics/${kind}${qs}`, '_blank')
  }

  const merged: Record<string, {
    input: number; output: number; cost: number; requests: number; tenants: Set<string>; currency: string
  }> = {}
  for (const r of tokenRecords) {
    if (!merged[r.model]) merged[r.model] = { input: 0, output: 0, cost: 0, requests: 0, tenants: new Set(), currency: 'USD' }
    merged[r.model].input += r.total_input_tokens
    merged[r.model].output += r.total_output_tokens
    merged[r.model].tenants.add(r.tenant_id)
  }
  for (const r of costRecords) {
    if (!merged[r.model]) merged[r.model] = { input: 0, output: 0, cost: 0, requests: 0, tenants: new Set(), currency: 'USD' }
    merged[r.model].cost += r.total_cost
    merged[r.model].requests += r.request_count
    merged[r.model].tenants.add(r.tenant_id)
    if (r.currency) merged[r.model].currency = r.currency
  }

  interface ModelRow { model: string; tenants: number; input: number; output: number; total: number; requests: number; cost: number; currency: string }
  const modelRows: ModelRow[] = Object.entries(merged).map(([model, m]) => ({
    model,
    tenants: m.tenants.size,
    input: m.input,
    output: m.output,
    total: m.input + m.output,
    requests: m.requests,
    cost: m.cost,
    currency: m.currency,
  }))
  const sort = useSort<ModelRow>('total', 'desc')
  const rows = useMemo(() => sortRows(modelRows, sort.key, sort.dir), [modelRows, sort])

  const tenantCost = useMemo(() => {
    const agg: Record<string, { cost: number; requests: number }> = {}
    for (const r of costRecords) {
      if (!agg[r.tenant_id]) agg[r.tenant_id] = { cost: 0, requests: 0 }
      agg[r.tenant_id].cost += r.total_cost
      agg[r.tenant_id].requests += r.request_count
    }
    return Object.entries(agg)
      .map(([tenant, v]) => ({ tenant, ...v }))
      .sort((a, b) => b.cost - a.cost)
      .slice(0, 10)
  }, [costRecords])

  const topModels = useMemo(() => {
    const agg: Record<string, number> = {}
    for (const r of costRecords) agg[r.model] = (agg[r.model] ?? 0) + r.request_count
    return Object.entries(agg)
      .map(([model, requests]) => ({ model, requests }))
      .sort((a, b) => b.requests - a.requests)
      .slice(0, 6)
  }, [costRecords])

  const rateData = useMemo(
    () => Object.entries(rateByModel)
      .map(([model, v]) => ({ model, ...v }))
      .sort((a, b) => (b.ok + b.error + b.blocked) - (a.ok + a.error + a.blocked)),
    [rateByModel],
  )

  const th = (k: keyof ModelRow, label: string, num = false) => (
    <th className={num ? 'num' : undefined}>
      <SortHeader active={sort.key === k} dir={sort.dir} onClick={() => sort.toggle(k)}>{label}</SortHeader>
    </th>
  )

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Usage Over Time</h3>
          <div className="header-actions">
            <button type="button" className="btn btn-small" onClick={() => exportCSV('tokens')}>Export CSV</button>
            <TimeRangePicker value={range} onChange={setRange} />
          </div>
        </div>
        <TimeSeriesChart data={series} height={220} />
      </div>

      <div className="stats-grid">
        <div className="stat-card"><div className="label">Total Tokens</div><div className="value">{fmtTokens(totalTokens)}</div></div>
        <div className="stat-card"><div className="label">Est. Cost</div><div className="value">${totalCost.toFixed(2)}</div></div>
        <div className="stat-card"><div className="label">Requests</div><div className="value">{totalRequests.toLocaleString()}</div></div>
        <div className="stat-card"><div className="label">Avg Tokens/Req</div><div className="value">{avgTokens}</div></div>
      </div>

      <div className="dash-grid">
        <div className="card">
          <h3>Cost by Tenant</h3>
          {tenantCost.length === 0 ? (
            <EmptyState message="No cost data" />
          ) : (
            <>
              <ResponsiveContainer width="100%" height={200}>
                <BarChart data={tenantCost} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                  <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
                  <XAxis dataKey="tenant" tick={{ fill: 'var(--text-muted)', fontSize: 10 }} tickLine={false} axisLine={{ stroke: 'var(--border)' }} />
                  <YAxis tickFormatter={(v: number) => `$${v.toFixed(0)}`} tick={{ fill: 'var(--text-muted)', fontSize: 10 }} tickLine={false} axisLine={false} width={44} />
                  <Tooltip formatter={(v) => `$${Number(v).toFixed(2)}`} cursor={{ fill: 'var(--row-hover)' }} />
                  <Bar dataKey="cost" fill="var(--accent)" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
              <div className="table-wrap">
                <table>
                  <tbody>
                    {tenantCost.slice(0, 5).map((t) => (
                      <tr key={t.tenant}>
                        <td><code>{t.tenant}</code></td>
                        <td className="num">${t.cost.toFixed(2)}</td>
                        <td className="num">{t.requests.toLocaleString()} req</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>

        <div className="card">
          <h3>Top Models by Requests</h3>
          {topModels.length === 0 ? (
            <EmptyState message="No traffic data" />
          ) : (
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={topModels} layout="vertical" margin={{ top: 4, right: 12, left: 0, bottom: 0 }}>
                <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" horizontal={false} />
                <XAxis type="number" tick={{ fill: 'var(--text-muted)', fontSize: 10 }} tickLine={false} axisLine={false} />
                <YAxis type="category" dataKey="model" width={110} tick={{ fill: 'var(--text-muted)', fontSize: 10 }} tickLine={false} axisLine={false} />
                <Tooltip formatter={(v) => Number(v).toLocaleString()} cursor={{ fill: 'var(--row-hover)' }} />
                <Bar dataKey="requests" fill="var(--green)" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>

        <div className="card" style={{ gridColumn: '1 / -1' }}>
          <h3>Request Status by Model <span className="text-muted" style={{ fontSize: 12, fontWeight: 400 }}>(last 100)</span></h3>
          {rateData.length === 0 ? (
            <EmptyState message="No conversation data yet" />
          ) : (
            <ResponsiveContainer width="100%" height={Math.max(120, rateData.length * 44)}>
              <BarChart data={rateData} layout="vertical" margin={{ top: 4, right: 12, left: 40, bottom: 0 }} barCategoryGap={6}>
                <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" horizontal={false} />
                <XAxis type="number" tick={{ fill: 'var(--text-muted)', fontSize: 10 }} tickLine={false} axisLine={false} />
                <YAxis type="category" dataKey="model" width={120} tick={{ fill: 'var(--text-muted)', fontSize: 10 }} tickLine={false} axisLine={false} />
                <Tooltip cursor={{ fill: 'var(--row-hover)' }} />
                <Bar dataKey="ok" name="OK" stackId="s" fill="var(--green)" />
                <Bar dataKey="blocked" name="Blocked" stackId="s" fill="var(--orange)" />
                <Bar dataKey="error" name="Error" stackId="s" fill="var(--red)" />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      <div className="card">
        <h3>Token Usage by Model</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                {th('model', 'Model')}
                {th('tenants', 'Tenants', true)}
                {th('input', 'Input', true)}
                {th('output', 'Output', true)}
                {th('total', 'Total', true)}
                {th('requests', 'Requests', true)}
                {th('cost', 'Cost', true)}
                <th>Currency</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((m) => (
                <tr key={m.model}>
                  <td>{m.model}</td>
                  <td className="num">{m.tenants}</td>
                  <td className="num">{fmtTokens(m.input)}</td>
                  <td className="num">{fmtTokens(m.output)}</td>
                  <td className="num">{fmtTokens(m.total)}</td>
                  <td className="num">{m.requests}</td>
                  <td className="num">{m.currency === 'USD' ? '$' : ''}{m.cost.toFixed(2)} {m.currency !== 'USD' ? m.currency : ''}</td>
                  <td>{m.currency}</td>
                </tr>
              ))}
              {!loading && Object.keys(merged).length === 0 && <tr><td colSpan={8}><EmptyState message="No data" /></td></tr>}
              {loading && <tr><td colSpan={8}><TableSkeleton rows={4} cols={8} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
