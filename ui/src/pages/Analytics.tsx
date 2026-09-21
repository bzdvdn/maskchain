import { useCallback, useEffect, useMemo, useState } from 'react'
import { Download } from 'lucide-react'
import { TimeRangePicker, useRange, usePersistedRange } from '../components/TimeRangePicker'
import { TimeSeriesChart } from '../components/TimeSeriesChart'
import { AsyncSection, Spinner, StatusPill, type StatusTone } from '../components/ui'
import { fmtTokens, groupNum, money, rangeSpanMs } from '../utils/format'
import {
  getAnalyticsCost,
  getAnalyticsSeries,
  getAnalyticsTokens,
  withTenant as withTenantParam,
} from '../api/analytics'

type Tab = 'day' | 'model' | 'tenant'

function shiftWindow(from: string, to: string): { from: string; to: string } {
  const f = new Date(from)
  const t = new Date(to)
  if (Number.isNaN(f.getTime()) || Number.isNaN(t.getTime())) return { from, to }
  const span = t.getTime() - f.getTime()
  return { from: new Date(f.getTime() - span).toISOString(), to: f.toISOString() }
}

function deltaPct(cur: number, prev: number): number | null {
  if (prev <= 0) return cur > 0 ? 100 : null
  return Math.round(((cur - prev) / prev) * 1000) / 10
}

function DeltaBadge({ delta }: { delta: number | null }) {
  if (delta === null) return <span className="muted">—</span>
  const tone: StatusTone = delta >= 0 ? 'green' : 'red'
  return <StatusPill tone={tone}>{delta >= 0 ? '▲' : '▼'} {Math.abs(delta)}%</StatusPill>
}

export function Analytics() {
  const [range, setRange] = usePersistedRange('maskchain.analytics.range')
  const { from, to } = useRange(range)
  const [workspace, setWorkspace] = useState<string>(() => localStorage.getItem('maskchain.workspace') ?? '')
  const [compare, setCompare] = useState(false)
  const [tab, setTab] = useState<Tab>('day')

  const [tokens, setTokens] = useState<{ records: any[]; totals: { total_input_tokens: number; total_output_tokens: number } }>({ records: [], totals: { total_input_tokens: 0, total_output_tokens: 0 } })
  const [cost, setCost] = useState<{ records: any[]; totals: { total_cost: number; request_count: number } }>({ records: [], totals: { total_cost: 0, request_count: 0 } })
  const [series, setSeries] = useState<{ bucket: string; input_tokens: number; output_tokens: number; cost: number; requests: number }[]>([])
  const [prev, setPrev] = useState({ total_cost: 0, request_count: 0, total_tokens: 0 })
  const [prevSeries, setPrevSeries] = useState<{ bucket: string; input_tokens: number; output_tokens: number }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)

  useEffect(() => {
    const onWorkspace = (e: Event) => setWorkspace((e as CustomEvent<string>).detail)
    window.addEventListener('maskchain:workspace', onWorkspace)
    return () => window.removeEventListener('maskchain:workspace', onWorkspace)
  }, [])

  const load = useCallback(() => {
    setLoading(true)
    setError(null)
    const prevWin = shiftWindow(from, to)
    return Promise.all([
      getAnalyticsTokens(from, to, workspace),
      getAnalyticsCost(from, to, workspace),
      getAnalyticsSeries(from, to, workspace),
      getAnalyticsCost(prevWin.from, prevWin.to, workspace),
      getAnalyticsSeries(prevWin.from, prevWin.to, workspace),
    ])
      .then(([t, c, s, pc, ps]) => {
        setTokens(t)
        setCost(c)
        setSeries(Array.isArray(s.series) ? s.series : [])
        const prevCost = pc.totals?.total_cost ?? 0
        const prevReq = pc.totals?.request_count ?? 0
        const prevToks = (ps?.series ?? []).reduce((sum, p) => sum + (p.input_tokens || 0) + (p.output_tokens || 0), 0)
        setPrev({ total_cost: prevCost, request_count: prevReq, total_tokens: prevToks })
        setPrevSeries(Array.isArray(ps.series) ? ps.series : [])
      })
      .catch((err) => setError(err))
      .finally(() => setLoading(false))
  }, [from, to, workspace])

  useEffect(() => {
    load()
  }, [load])

  const totalTokens = (tokens.totals?.total_input_tokens ?? 0) + (tokens.totals?.total_output_tokens ?? 0)
  const totalCost = cost.totals?.total_cost ?? 0
  const totalRequests = cost.totals?.request_count ?? 0
  const avgTokens = totalRequests > 0 ? Math.round(totalTokens / totalRequests) : 0

  const modelRows = useMemo(() => {
    const merged: Record<string, { tenants: Set<string>; input: number; output: number; requests: number; cost: number }> = {}
    for (const r of tokens.records ?? []) {
      const m = r.model || 'unknown'
      if (!merged[m]) merged[m] = { tenants: new Set(), input: 0, output: 0, requests: 0, cost: 0 }
      merged[m].input += r.total_input_tokens || 0
      merged[m].output += r.total_output_tokens || 0
      merged[m].tenants.add(r.tenant_id)
    }
    for (const r of cost.records ?? []) {
      const m = r.model || 'unknown'
      if (!merged[m]) merged[m] = { tenants: new Set(), input: 0, output: 0, requests: 0, cost: 0 }
      merged[m].requests += r.request_count || 0
      merged[m].cost += r.total_cost || 0
      merged[m].tenants.add(r.tenant_id)
    }
    return Object.entries(merged)
      .map(([model, v]) => ({ model, tenants: v.tenants.size, input: v.input, output: v.output, total: v.input + v.output, requests: v.requests, cost: v.cost }))
      .sort((a, b) => b.cost - a.cost)
  }, [tokens.records, cost.records])

  const tenantRows = useMemo(() => {
    const merged: Record<string, { cost: number; requests: number }> = {}
    for (const r of cost.records ?? []) {
      if (!merged[r.tenant_id]) merged[r.tenant_id] = { cost: 0, requests: 0 }
      merged[r.tenant_id].cost += r.total_cost || 0
      merged[r.tenant_id].requests += r.request_count || 0
    }
    return Object.entries(merged)
      .map(([tenant, v]) => ({ tenant, ...v }))
      .sort((a, b) => b.cost - a.cost)
  }, [cost.records])

  const exportCSV = useCallback((kind: 'tokens' | 'cost' | 'timeseries') => {
    const base = `/api/v1/analytics/${kind}?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&format=csv`
    window.open(withTenantParam(base, workspace), '_blank')
  }, [from, to, workspace])

  return (
    <div>
      <div className="card tight u-mb16">
        <div className="toolbar u-mb0">
          <span className="toolbar-label">Range</span>
          <TimeRangePicker value={range} onChange={setRange} />
          <span className="vsep" />
          <span className="toolbar-label">Compare</span>
          <button
            type="button"
            role="switch"
            aria-checked={compare}
            className={`mc-switch${compare ? ' on' : ''}`}
            aria-label="Compare to previous period"
            onClick={() => setCompare((c) => !c)}
          />
          <span className="toolbar-hint">vs previous period</span>
          <span className="u-grow" />
          <button className="btn btn-small" onClick={() => exportCSV('cost')}><Download size={13} /> CSV</button>
        </div>
      </div>

      <div className="stats-grid">
        {[
          { label: 'Estimated cost', value: money(totalCost), delta: deltaPct(totalCost, prev.total_cost) },
          { label: 'Requests', value: groupNum(totalRequests), delta: deltaPct(totalRequests, prev.request_count) },
          { label: 'Tokens', value: fmtTokens(totalTokens), delta: deltaPct(totalTokens, prev.total_tokens) },
          { label: 'Avg tokens / request', value: groupNum(avgTokens), delta: null },
        ].map((kpi) => (
          <div key={kpi.label} className="stat-card">
            <div className="label">{kpi.label}</div>
            <div className="value">{kpi.value}</div>
            <div className="change u-mt4">{kpi.delta !== null && <DeltaBadge delta={kpi.delta} />}</div>
          </div>
        ))}
      </div>

      <div className="tabs" role="tablist">
        {(['day', 'model', 'tenant'] as Tab[]).map((t) => (
          <button key={t} role="tab" aria-selected={tab === t} className={`tab${tab === t ? ' active' : ''}`} onClick={() => setTab(t)}>
            {t === 'day' ? 'By day' : t === 'model' ? 'By model' : 'By tenant'}
          </button>
        ))}
      </div>

      {tab === 'day' && (
        <div className="card">
          <div className="card-header-row">
            <h3>Usage over time</h3>
            {compare && <StatusPill tone="blue">compare on</StatusPill>}
          </div>
          <AsyncSection
            loading={loading && series.length === 0}
            error={series.length === 0 ? error : null}
            onRetry={load}
            empty={!loading && series.length === 0}
            emptyMessage="No data for this range."
            skeleton={<Spinner label="Loading…" />}
          >
            <TimeSeriesChart data={series} height={240} compare={compare ? prevSeries : undefined} spanMs={rangeSpanMs(range.mode, from, to)} />
          </AsyncSection>
        </div>
      )}

      {tab === 'model' && (
        <div className="card table-card">
          <div className="table-wrap table-flush">
            <table className="tbl">
              <thead>
                <tr><th>Model</th><th className="num">Tenants</th><th className="num">Input</th><th className="num">Output</th><th className="num">Total</th><th className="num">Requests</th><th className="num">Cost</th></tr>
              </thead>
              <AsyncSection
                as="tbody"
                colSpan={7}
                loading={loading}
                error={error}
                onRetry={load}
                empty={modelRows.length === 0}
                emptyMessage="No data"
              >
                {modelRows.map((m) => (
                  <tr key={m.model}>
                    <td className="mono">{m.model}</td>
                    <td className="num">{m.tenants}</td>
                    <td className="num">{fmtTokens(m.input)}</td>
                    <td className="num">{fmtTokens(m.output)}</td>
                    <td className="num">{fmtTokens(m.total)}</td>
                    <td className="num">{groupNum(m.requests)}</td>
                    <td className="num">{money(m.cost)}</td>
                  </tr>
                ))}
              </AsyncSection>
            </table>
          </div>
        </div>
      )}

      {tab === 'tenant' && (
        <div className="card table-card">
          <div className="table-wrap table-flush">
            <table className="tbl">
              <thead>
                <tr><th>Tenant</th><th className="num">Requests</th><th className="num">Cost</th></tr>
              </thead>
              <AsyncSection
                as="tbody"
                colSpan={3}
                loading={loading}
                error={error}
                onRetry={load}
                empty={tenantRows.length === 0}
                emptyMessage="No data"
              >
                {tenantRows.map((t) => (
                  <tr key={t.tenant}>
                    <td className="mono">{t.tenant}</td>
                    <td className="num">{groupNum(t.requests)}</td>
                    <td className="num">{money(t.cost)}</td>
                  </tr>
                ))}
              </AsyncSection>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}