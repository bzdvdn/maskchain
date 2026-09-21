import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { BarChart3, KeyRound, Shield, Users, Wallet, Zap, AlertTriangle, Plus } from 'lucide-react'
import { TimeRangePicker, useRange, defaultRange, type RangeValue } from '../components/TimeRangePicker'
import { Segmented, StatusPill, Button, type StatusTone } from '../components/ui'
import { TimeSeriesChart, type ChartMetric } from '../components/TimeSeriesChart'
import { relativeTime, money, fmtTokens, groupNum, rangeLabel, rangeSpanMs } from '../utils/format'
import {
  getAnalyticsTokens,
  getAnalyticsCost,
  getAnalyticsSeries,
  type SeriesPoint,
} from '../api/analytics'
import { listBudgets, listBudgetsByTenant } from '../api/budgets'
import { listProviders } from '../api/routing'
import { listKeys } from '../api/keys'
import { listConversations } from '../api/conversations'
import { useGatewayBase } from '../hooks/useGatewayBase'

type Metric = ChartMetric

const METRICS: { key: Metric; label: string }[] = [
  { key: 'tokens', label: 'Tokens' },
  { key: 'cost', label: 'Cost' },
  { key: 'requests', label: 'Requests' },
]

const METRIC_LABEL: Record<Metric, string> = {
  tokens: 'Tokens',
  cost: 'Cost',
  requests: 'Requests',
}

interface AttentionItem {
  tone: StatusTone
  title: string
  detail: string
  to?: string
}

function shiftWindow(from: string, to: string): { from: string; to: string } {
  const f = new Date(from)
  const t = new Date(to)
  if (Number.isNaN(f.getTime()) || Number.isNaN(t.getTime())) {
    return { from, to }
  }
  const span = t.getTime() - f.getTime()
  return { from: new Date(f.getTime() - span).toISOString(), to: f.toISOString() }
}

function deltaPct(cur: number, prev: number): number | null {
  if (prev <= 0) return cur > 0 ? 100 : null
  return Math.round(((cur - prev) / prev) * 1000) / 10
}

function seriesValue(p: SeriesPoint, metric: Metric): number {
  if (metric === 'cost') return p.cost
  if (metric === 'requests') return p.requests
  return p.input_tokens + p.output_tokens
}

function Sparkline({ points, metric }: { points: SeriesPoint[]; metric: Metric }) {
  const values = points.map((p) => seriesValue(p, metric))
  if (values.length === 0 || values.every((v) => v === 0)) {
    // No activity: a neutral baseline reads better than a flat accent line.
    return (
      <svg viewBox="0 0 120 30" preserveAspectRatio="none" width="100%" height="30" className="spark-svg" aria-hidden="true">
        <line x1="0" y1="29" x2="120" y2="29" stroke="var(--border)" strokeWidth="1.5" strokeDasharray="3 3" />
      </svg>
    )
  }
  const max = Math.max(1, ...values)
  const step = 120 / Math.max(1, values.length - 1)
  const d = values.map((v, i) => `${i === 0 ? 'M' : 'L'}${i * step},${30 - (v / max) * 26}`).join(' ')
  return (
    <svg viewBox="0 0 120 30" preserveAspectRatio="none" width="100%" height="30" className="spark-svg" aria-hidden="true">
      <path d={d} fill="none" stroke="var(--accent)" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  )
}

export function Dashboard() {
  const navigate = useNavigate()
  const gatewayBase = useGatewayBase()
  const [range, setRange] = useState<RangeValue>(defaultRange)
  const { from, to } = useRange(range)
  const [metric, setMetric] = useState<Metric>('tokens')
  const [workspace, setWorkspace] = useState<string>(() => localStorage.getItem('maskchain.workspace') ?? '')

  const [totals, setTotals] = useState({ total_input_tokens: 0, total_output_tokens: 0 })
  const [totalCost, setTotalCost] = useState(0)
  const [totalRequests, setTotalRequests] = useState(0)
  const [series, setSeries] = useState<SeriesPoint[]>([])
  const [seriesError, setSeriesError] = useState(false)
  const [prevTotals, setPrevTotals] = useState({ total_output_tokens: 0, total_input_tokens: 0, total_cost: 0 })
  const [prevRequests, setPrevRequests] = useState(0)
  const [passRate, setPassRate] = useState(0)
  const [attention, setAttention] = useState<AttentionItem[]>([])
  const [activity, setActivity] = useState<{ id: string; tenant_id: string; model: string; status: string; created_at: string }[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    const onWorkspace = (e: Event) => setWorkspace((e as CustomEvent<string>).detail)
    window.addEventListener('maskchain:workspace', onWorkspace)
    return () => window.removeEventListener('maskchain:workspace', onWorkspace)
  }, [])

  const loadKpis = useCallback(async () => {
    const prev = shiftWindow(from, to)
    try {
      const [curToks, curCost, prevToks, prevCost] = await Promise.all([
        getAnalyticsTokens(from, to, workspace),
        getAnalyticsCost(from, to, workspace),
        getAnalyticsTokens(prev.from, prev.to, workspace),
        getAnalyticsCost(prev.from, prev.to, workspace),
      ])
      setTotals(curToks.totals)
      setTotalCost(curCost.totals?.total_cost ?? 0)
      setTotalRequests(curCost.totals?.request_count ?? 0)
      setPrevTotals({
        total_input_tokens: prevToks.totals?.total_input_tokens ?? 0,
        total_output_tokens: prevToks.totals?.total_output_tokens ?? 0,
        total_cost: prevCost.totals?.total_cost ?? 0,
      })
      setPrevRequests(prevCost.totals?.request_count ?? 0)
      setError('')
    } catch {
      setError('No data yet')
    }
  }, [from, to, workspace])

  const loadSeries = useCallback(async () => {
    try {
      const d = await getAnalyticsSeries(from, to, workspace)
      setSeries(Array.isArray(d.series) ? d.series : [])
      setSeriesError(false)
    } catch {
      setSeries([])
      setSeriesError(true)
    }
  }, [from, to, workspace])

  const loadAttention = useCallback(async () => {
    const items: AttentionItem[] = []
    try {
      const budgets = workspace ? await listBudgetsByTenant(workspace) : await listBudgets()
      const list = budgets?.data ?? budgets ?? []
      for (const b of list) {
        if (!b.hard_limit) continue
        const pct = (b.spent / b.hard_limit) * 100
        if (pct >= 100) {
          items.push({ tone: 'red', title: 'Budget exhausted', detail: `${b.tenant_id} hit ${money(b.hard_limit)} / ${b.type}`, to: '/budgets' })
        } else if (pct >= 70) {
          items.push({ tone: 'amber', title: `Budget at ${Math.round(pct)}%`, detail: `${b.tenant_id} — ${money(b.spent)} of ${money(b.hard_limit)}`, to: '/budgets' })
        }
      }
    } catch {
      /* attention panel degrades gracefully */
    }
    try {
      const providers = await listProviders()
      for (const p of Array.isArray(providers) ? providers : []) {
        if (p.status === 'down') items.push({ tone: 'red', title: 'Provider down', detail: `${p.name} unreachable`, to: '/routing' })
        else if (p.status === 'degraded') items.push({ tone: 'amber', title: 'Provider degraded', detail: p.name, to: '/routing' })
      }
    } catch {
      /* ignore */
    }
    try {
      const keys = await listKeys()
      const rows = Array.isArray(keys?.data) ? keys.data : Array.isArray(keys) ? keys : []
      const expiring = rows.filter((k) => k.expires_at && new Date(k.expires_at).getTime() - Date.now() < 7 * 86400_000 && new Date(k.expires_at).getTime() > Date.now())
      if (expiring.length > 0) {
        items.push({ tone: 'blue', title: 'Keys expiring', detail: `${expiring.length} virtual key${expiring.length > 1 ? 's' : ''} expire within 7 days`, to: '/keys' })
      }
    } catch {
      /* ignore */
    }
    try {
      const conv = await listConversations(1, 100)
      const convRows = Array.isArray(conv.items) ? conv.items : []
      const blocked = convRows.filter((c) => c.status === 'blocked').length
      if (blocked > 0) items.push({ tone: 'amber', title: 'Requests blocked', detail: `${blocked} messages blocked · last 100`, to: '/conversations' })
    } catch {
      /* ignore */
    }
    setAttention(items.slice(0, 5))
  }, [workspace])

  const loadActivity = useCallback(async () => {
    try {
      const conv = await listConversations(1, 100)
      const rows = Array.isArray(conv.items) ? conv.items : []
      setActivity(rows.slice(0, 6).map((c) => ({ id: c.id, tenant_id: c.tenant_id, model: c.model, status: c.status, created_at: c.created_at })))
    } catch {
      setActivity([])
    }
  }, [])

  useEffect(() => {
    loadKpis()
    loadAttention()
    loadActivity()
  }, [loadKpis, loadAttention, loadActivity])

  useEffect(() => {
    loadSeries()
  }, [loadSeries, metric])

  useEffect(() => {
    let active = true
    listConversations(1, 100, workspace ? { tenant_id: workspace } : {})
      .then((conv) => {
        if (!active) return
        const rows = Array.isArray(conv.items) ? conv.items : []
        const ok = rows.filter((c) => c.status === 'ok').length
        setPassRate(rows.length > 0 ? Math.round((ok / rows.length) * 1000) / 10 : 0)
      })
      .catch(() => active && setPassRate(0))
    return () => {
      active = false
    }
  }, [workspace])

  const tokensTotal = totals.total_input_tokens + totals.total_output_tokens
  const tokensDelta = deltaPct(tokensTotal, prevTotals.total_input_tokens + prevTotals.total_output_tokens)
  const costDelta = deltaPct(totalCost, prevTotals.total_cost)
  const requestsDelta = deltaPct(totalRequests, prevRequests)

  const firstRun = series.length === 0 && tokensTotal === 0 && attention.length === 0 && activity.length === 0 && !error

  const scope = workspace ? ` · ${workspace}` : ''
  const rLabel = rangeLabel(range.mode, from, to)
  const cards: { key: string; label: string; value: string; delta: number | null; metric: Metric | null }[] = [
    { key: 'spend', label: `Spend · ${rLabel}${scope}`, value: money(totalCost), delta: costDelta, metric: 'cost' },
    { key: 'requests', label: `Requests · ${rLabel}${scope}`, value: groupNum(totalRequests), delta: requestsDelta, metric: 'requests' },
    { key: 'tokens', label: `Tokens · ${rLabel}${scope}`, value: fmtTokens(tokensTotal), delta: tokensDelta, metric: 'tokens' },
    { key: 'pass', label: `Pass rate · last 100${scope || ' · all tenants'}`, value: `${passRate}%`, delta: null, metric: null },
  ]

  const trendTotal = metric === 'cost' ? money(totalCost) : metric === 'requests' ? groupNum(totalRequests) : fmtTokens(tokensTotal)
  const trendDelta = metric === 'cost' ? costDelta : metric === 'requests' ? requestsDelta : tokensDelta

  return (
    <div>
      {firstRun ? (
        <div className="onboard">
          <div className="onboard-title">Welcome to MaskChain</div>
          <p className="muted-sm">Your privacy-safe LLM gateway is up. Issue a virtual key and route your first request.</p>
          <div className="codebox u-block">
            curl {gatewayBase || '…'}/api/v1/chat/completions \<br />
            &nbsp;&nbsp;-H "Authorization: Bearer sk-mc-…" \<br />
            &nbsp;&nbsp;-d {'{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}'}
          </div>
          <div className="u-flex">
            <Button variant="primary" onClick={() => navigate('/keys?create=1')}><Plus size={14} /> Create virtual key</Button>
            <Button onClick={() => navigate('/tenants/new')}><Users size={14} /> Add tenant</Button>
          </div>
        </div>
      ) : (
        <>
      <div className="card u-mb24">
        <div className="card-header-row">
          <h3>Traffic trend · {METRIC_LABEL[metric]} · {rLabel}{scope}</h3>
          <div className="u-flex-lg">
            <Segmented<Metric> options={METRICS} value={metric} onChange={setMetric} ariaLabel="Trend metric" />
            <TimeRangePicker value={range} onChange={setRange} />
          </div>
        </div>
        <div className="muted-sm u-mb8">
          Total: {trendTotal}
          {trendDelta !== null && ` · ${trendDelta >= 0 ? '▲' : '▼'} ${Math.abs(trendDelta)}% vs previous period`}
        </div>
        {seriesError ? (
          <p className="text-muted">Could not load the trend for this range. Try again.</p>
        ) : (
          <TimeSeriesChart data={series} metric={metric} spanMs={rangeSpanMs(range.mode, from, to)} />
        )}
        <div className="muted-sm u-mt8">
          Tokens = input + output · Cost in the configured currency · Requests from usage records
        </div>
      </div>

      <div className="stats-grid">
        {cards.map((card) => (
          <div key={card.label} className="stat-card">
            <div className="label">{card.label}</div>
            <div className="value">{card.value}</div>
            {card.delta !== null && (
              <div className={`change ${card.delta >= 0 ? 'up' : 'down'}`}>
                {card.delta >= 0 ? '▲' : '▼'} {Math.abs(card.delta)}%
              </div>
            )}
            {card.metric && (
              <div className="u-mt6">
                <Sparkline points={series} metric={card.metric} />
              </div>
            )}
          </div>
        ))}
      </div>

      {error && <p className="text-muted u-mb24">{error}</p>}

      <div className="dash-grid">
        <div className="card">
          <div className="card-header-row">
            <h3>Needs attention</h3>
            {attention.length > 0 && <StatusPill tone={attention.some((a) => a.tone === 'red') ? 'red' : 'amber'}>{attention.length}</StatusPill>}
          </div>
          {attention.length === 0 ? (
            <div className="empty-state u-center">All systems nominal.</div>
          ) : (
            <ul className="attn-list">
              {attention.map((a, i) => (
                <li key={`${a.title}-${i}`} className="attn-row">
                  <StatusPill tone={a.tone}>{a.tone === 'red' ? 'critical' : a.tone}</StatusPill>
                  <div className="u-grow">
                    <div className="attn-title">{a.title}</div>
                    <div className="attn-detail u-muted">{a.detail}</div>
                  </div>
                  {a.to && <Link className="btn-link attn-open" to={a.to}>Open →</Link>}
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="card">
          <div className="card-header-row"><h3>Quick actions</h3></div>
          <div className="u-col">
            <button className="btn" onClick={() => navigate('/keys?create=1')}><KeyRound size={14} /> Create virtual key</button>
            <button className="btn" onClick={() => navigate('/tenants/new')}><Users size={14} /> Add tenant</button>
            <button className="btn" onClick={() => navigate('/budgets')}><Wallet size={14} /> Set budget</button>
            <button className="btn" onClick={() => navigate('/routing')}><BarChart3 size={14} /> Inspect routing</button>
          </div>
        </div>
      </div>

      <div className="card">
        <div className="card-header-row">
          <h3>Latest activity</h3>
          <Link className="btn-link link-sm" to="/conversations">View all</Link>
        </div>
        {activity.length === 0 ? (
          <div className="empty-state u-center">No traffic yet.</div>
        ) : (
          <div className="table-wrap">
            <table className="tbl">
              <thead>
                <tr><th>Event</th><th>Tenant</th><th>Model</th><th>Status</th><th>When</th></tr>
              </thead>
              <tbody>
                {activity.map((a) => (
                  <tr key={a.id}>
                    <td>
                      <span className="u-flex">
                        {a.status === 'blocked' ? <Shield size={13} className="ic-accent" /> : a.status === 'ok' ? <Zap size={13} className="ic-green" /> : <AlertTriangle size={13} className="ic-amber" />}
                        {a.status === 'blocked' ? 'Mask blocked' : a.status === 'ok' ? 'Request passed' : 'Request failed'}
                      </span>
                    </td>
                    <td><code>{a.tenant_id}</code></td>
                    <td>{a.model || '—'}</td>
                    <td><StatusPill tone={a.status === 'ok' ? 'green' : a.status === 'blocked' ? 'amber' : 'red'}>{a.status}</StatusPill></td>
                    <td className="muted">{relativeTime(a.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
        </>
      )}
    </div>
  )
}