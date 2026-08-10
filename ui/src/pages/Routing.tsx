import { useMemo } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Badge, EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { useSort, sortRows } from '../hooks/useSort'

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

interface RoutingData {
  providers: Provider[]
  model_routes: Rule[]
}

function timeAgo(unix: number | undefined): string {
  if (!unix) return '—'
  const sec = Math.floor((Date.now() / 1000) - unix)
  if (sec < 0) return 'now'
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
}

async function fetchRouting(): Promise<RoutingData> {
  const token = localStorage.getItem('admin_token')
  const res = await fetch('/api/v1/routing', {
    headers: token ? { 'Authorization': `Bearer ${token}` } : {},
    credentials: 'include',
  })
  if (!res.ok) throw new Error('fetch failed')
  const body = await res.json()
  const d = body.data ?? body
  return {
    providers: Array.isArray(d.providers) ? d.providers : [],
    model_routes: Array.isArray(d.model_routes) ? d.model_routes : [],
  }
}

export function Routing() {
  const { data, loading } = useAsyncData(fetchRouting, [])
  const providers = data?.providers ?? []
  const rules = data?.model_routes ?? []

  const provSort = useSort<Provider>('name', 'asc')
  const provRows = useMemo(() => sortRows(providers, provSort.key, provSort.dir), [providers, provSort])
  const ruleSort = useSort<Rule>('model', 'asc')
  const ruleRows = useMemo(() => sortRows(rules, ruleSort.key, ruleSort.dir), [rules, ruleSort])

  const provTh = (k: keyof Provider, label: string, num = false) => (
    <th className={num ? 'num' : undefined}>
      <SortHeader active={provSort.key === k} dir={provSort.dir} onClick={() => provSort.toggle(k)}>{label}</SortHeader>
    </th>
  )

  return (
    <div>
      <div className="card">
        <h3>Providers</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                {provTh('name', 'Name')}
                {provTh('api_type', 'Type')}
                <th>Base URL</th>
                {provTh('status', 'Status')}
                {provTh('latency_ms', 'Latency', true)}
                <th>Last Check</th>
              </tr>
            </thead>
            <tbody>
              {provRows.map((p, i) => (
                <tr key={i}>
                  <td>{p.name}</td><td>{p.api_type}</td>
                  <td><code>{p.base_url}</code></td>
                  <td><Badge value={p.status} /></td>
                  <td className="num">{p.latency_ms != null ? `${p.latency_ms}ms` : '—'}</td>
                  <td>{timeAgo(p.last_check)}</td>
                </tr>
              ))}
              {!loading && provRows.length === 0 && <tr><td colSpan={6}><EmptyState message="No providers configured" /></td></tr>}
              {loading && <tr><td colSpan={6}><TableSkeleton rows={4} cols={6} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>
      <div className="card">
        <h3>Routing Rules</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>
                  <SortHeader active={ruleSort.key === 'model'} dir={ruleSort.dir} onClick={() => ruleSort.toggle('model')}>Model</SortHeader>
                </th>
                <th>Tenants</th>
                <th>Providers</th>
              </tr>
            </thead>
            <tbody>
              {ruleRows.map((r, i) => (
                <tr key={i}>
                  <td><code>{r.model}</code></td>
                  <td>{r.tenants.join(', ')}</td>
                  <td><code>{r.providers.join(', ')}</code></td>
                </tr>
              ))}
              {!loading && ruleRows.length === 0 && <tr><td colSpan={3}><EmptyState message="No routing rules" /></td></tr>}
              {loading && <tr><td colSpan={3}><TableSkeleton rows={4} cols={3} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
