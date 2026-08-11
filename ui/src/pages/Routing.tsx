import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Badge, Button, EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/Toast'
import { useSort, sortRows } from '../hooks/useSort'
import {
  deleteCostRate,
  deleteProvider,
  deleteRoute,
  listCostRates,
  listProviders,
  listRoutes,
  upsertCostRate,
  upsertProvider,
  upsertRoute,
  type CostRateDto,
  type ProviderDto,
  type RouteDto,
} from '../api/routing'

function timeAgo(unix: number | undefined): string {
  if (!unix) return '—'
  const sec = Math.floor((Date.now() / 1000) - unix)
  if (sec < 0) return 'now'
  if (sec < 60) return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
}

function SourceBadge({ source }: { source?: string }) {
  if (source === 'yaml') return <Badge value="yaml" />
  if (source === 'ui') return <Badge value="ui" />
  return <span className="text-muted">—</span>
}

export function Routing() {
  const { toast } = useToast()
  const [providers, setProviders] = useState<ProviderDto[]>([])
  const [rules, setRules] = useState<RouteDto[]>([])
  const [rates, setRates] = useState<CostRateDto[]>([])
  const [loading, setLoading] = useState(true)

  const reload = async () => {
    setLoading(true)
    try {
      const [p, r, c] = await Promise.all([listProviders(), listRoutes(), listCostRates()])
      setProviders(p ?? [])
      setRules(r ?? [])
      setRates(c ?? [])
    } catch {
      toast('Failed to load routing data', 'error')
    } finally {
      setLoading(false)
    }
  }

  useAsyncData(async () => {
    await reload()
    return null
  }, [])

  const providerSort = useSort<ProviderDto>('name', 'asc')
  const providerRows = useMemo(() => sortRows(providers, providerSort.key, providerSort.dir), [providers, providerSort])
  const ruleSort = useSort<RouteDto>('model', 'asc')
  const ruleRows = useMemo(() => sortRows(rules, ruleSort.key, ruleSort.dir), [rules, ruleSort])
  const rateSort = useSort<CostRateDto>('model', 'asc')
  const rateRows = useMemo(() => sortRows(rates, rateSort.key, rateSort.dir), [rates, rateSort])

  const [editingProvider, setEditingProvider] = useState<ProviderDto | null>(null)
  const [editingRoute, setEditingRoute] = useState<RouteDto | null>(null)
  const [editingRate, setEditingRate] = useState<CostRateDto | null>(null)
  const [deleting, setDeleting] = useState<null | { kind: 'provider' | 'route' | 'rate'; name: string; payload?: unknown }>(null)
  const [busy, setBusy] = useState(false)

  const availableTypes = ['openai', 'anthropic', 'ollama', 'proxy', 'gemini', 'bedrock']

  const providerTh = (k: keyof ProviderDto, label: string, num = false) => (
    <th className={num ? 'num' : undefined}>
      <SortHeader active={providerSort.key === k} dir={providerSort.dir} onClick={() => providerSort.toggle(k)}>{label}</SortHeader>
    </th>
  )

  const doDelete = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      if (deleting.kind === 'provider') {
        await deleteProvider(deleting.name)
        toast(`Provider "${deleting.name}" deleted`, 'success')
      } else if (deleting.kind === 'route') {
        await deleteRoute(deleting.payload as RouteDto)
        toast(`Route "${deleting.name}" deleted`, 'success')
      } else {
        await deleteCostRate(deleting.name)
        toast(`Cost rate "${deleting.name}" deleted`, 'success')
      }
      setDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Delete failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Providers</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditingProvider({} as ProviderDto)}>Add Provider</Button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                {providerTh('name', 'Name')}
                {providerTh('api_type', 'Type')}
                <th>Base URL</th>
                <th>Source</th>
                {providerTh('status', 'Status')}
                {providerTh('latency_ms', 'Latency', true)}
                <th>Last Check</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {providerRows.map((p, i) => (
                <tr key={i}>
                  <td>{p.name}</td><td>{p.api_type}</td>
                  <td><code>{p.base_url}</code></td>
                  <td><SourceBadge source={p.source} /></td>
                  <td><Badge value={p.status ?? 'unknown'} /></td>
                  <td className="num">{p.latency_ms != null ? `${p.latency_ms}ms` : '—'}</td>
                  <td>{timeAgo(p.last_check)}</td>
                  <td>
                    <div className="header-actions">
                      <Button size="small" onClick={() => setEditingProvider(p)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting({ kind: 'provider', name: p.name })}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && providerRows.length === 0 && <tr><td colSpan={8}><EmptyState message="No providers configured" /></td></tr>}
              {loading && <tr><td colSpan={8}><TableSkeleton rows={4} cols={8} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      <div className="card">
        <div className="card-header-row">
          <h3>Routing Rules</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditingRoute({ tenant: '', model: '', providers: [] })}>Add Rule</Button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>
                  <SortHeader active={ruleSort.key === 'model'} dir={ruleSort.dir} onClick={() => ruleSort.toggle('model')}>Model</SortHeader>
                </th>
                <th>Tenant</th>
                <th>Providers</th>
                <th>Source</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {ruleRows.map((r, i) => (
                <tr key={i}>
                  <td><code>{r.model}</code></td>
                  <td>{r.tenant}</td>
                  <td><code>{r.providers.join(', ')}</code></td>
                  <td><SourceBadge source={r.source} /></td>
                  <td>
                    <div className="header-actions">
                      <Button size="small" onClick={() => setEditingRoute(r)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting({ kind: 'route', name: `${r.tenant}/${r.model}`, payload: r })}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && ruleRows.length === 0 && <tr><td colSpan={5}><EmptyState message="No routing rules" /></td></tr>}
              {loading && <tr><td colSpan={5}><TableSkeleton rows={4} cols={5} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      <div className="card">
        <div className="card-header-row">
          <h3>Cost Rates</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditingRate({ model: '', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD' })}>Add Cost Rate</Button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>
                  <SortHeader active={rateSort.key === 'model'} dir={rateSort.dir} onClick={() => rateSort.toggle('model')}>Model</SortHeader>
                </th>
                <th>Input / 1K</th>
                <th>Output / 1K</th>
                <th>Currency</th>
                <th>Source</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rateRows.map((r, i) => (
                <tr key={i}>
                  <td><code>{r.model}</code></td>
                  <td className="num">{r.input_price_per_1k}</td>
                  <td className="num">{r.output_price_per_1k}</td>
                  <td>{r.currency || 'USD'}</td>
                  <td><SourceBadge source={r.source} /></td>
                  <td>
                    <div className="header-actions">
                      <Button size="small" onClick={() => setEditingRate(r)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting({ kind: 'rate', name: r.model })}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && rateRows.length === 0 && <tr><td colSpan={6}><EmptyState message="No cost rates configured" /></td></tr>}
              {loading && <tr><td colSpan={6}><TableSkeleton rows={4} cols={6} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {(editingProvider || editingRoute || editingRate) && (
        <CrudModal
          kind={editingProvider ? 'provider' : editingRoute ? 'route' : 'rate'}
          availableTypes={availableTypes}
          initialProvider={editingProvider}
          initialRoute={editingRoute}
          initialRate={editingRate}
          onClose={() => { setEditingProvider(null); setEditingRoute(null); setEditingRate(null) }}
          onSaved={async (kind, payload) => {
            try {
              if (kind === 'provider') {
                await upsertProvider(payload as ProviderDto)
                toast('Provider saved', 'success')
              } else if (kind === 'route') {
                await upsertRoute(payload as RouteDto)
                toast('Rule saved', 'success')
              } else {
                await upsertCostRate(payload as CostRateDto)
                toast('Cost rate saved', 'success')
              }
              setEditingProvider(null); setEditingRoute(null); setEditingRate(null)
              await reload()
            } catch (e: any) {
              toast(e?.message ?? 'Save failed', 'error')
            }
          }}
        />
      )}

      <ConfirmModal
        open={!!deleting}
        title={deleting?.kind === 'provider' ? 'Delete provider' : deleting?.kind === 'route' ? 'Delete rule' : 'Delete cost rate'}
        message={`Delete "${deleting?.name}"? This cannot be undone.`}
        busy={busy}
        onConfirm={doDelete}
        onCancel={() => setDeleting(null)}
      />
    </div>
  )
}

function CrudModal({
  kind,
  availableTypes,
  initialProvider,
  initialRoute,
  initialRate,
  onClose,
  onSaved,
}: {
  kind: 'provider' | 'route' | 'rate'
  availableTypes: string[]
  initialProvider: ProviderDto | null
  initialRoute: RouteDto | null
  initialRate: CostRateDto | null
  onClose: () => void
  onSaved: (kind: 'provider' | 'route' | 'rate', payload: unknown) => void
}) {
  const baseProvider = initialProvider ?? { name: '', api_type: 'openai', base_url: '', api_keys: [] } as ProviderDto
  const [p, setP] = useState<ProviderDto>({ ...baseProvider, api_keys: baseProvider.api_keys ?? [] })
  const [r, setR] = useState<RouteDto>(initialRoute ?? { tenant: '', model: '', providers: [] })
  const [c, setC] = useState<CostRateDto>(initialRate ?? { model: '', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD' })
  const [err, setErr] = useState('')

  const set = (updater: React.SetStateAction<ProviderDto>) => setP(prev => typeof updater === 'function' ? updater(prev) : updater)

  const submit = () => {
    setErr('')
    if (kind === 'provider') {
      if (!p.name || !p.base_url) { setErr('Name and Base URL are required'); return }
      if ((p.api_keys ?? []).length === 0) { setErr('At least one API key is required'); return }
      onSaved('provider', p)
    } else if (kind === 'route') {
      if (!r.model || r.providers.length === 0) { setErr('Model and at least one provider are required'); return }
      onSaved('route', r)
    } else {
      if (!c.model) { setErr('Model is required'); return }
      onSaved('rate', { model: c.model, input_price_per_1k: Number(c.input_price_per_1k), output_price_per_1k: Number(c.output_price_per_1k), currency: c.currency })
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal generic-modal" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <h3>{kind === 'provider' ? (initialProvider ? 'Edit Provider' : 'Add Provider') : kind === 'route' ? (initialRoute ? 'Edit Rule' : 'Add Rule') : (initialRate ? 'Edit Cost Rate' : 'Add Cost Rate')}</h3>
        {err && <div className="confirm-dialog" style={{ marginTop: 8 }}><p>{err}</p></div>}
        <div className="modal-body">
          {kind === 'provider' && (
            <>
              <div className="form-field">
                <label>Name</label>
                <input value={p.name} onChange={(e) => set({ ...p, name: e.target.value })} placeholder="my-provider" />
              </div>
              <div className="form-field">
                <label>API Type</label>
                <select value={p.api_type} onChange={(e) => set({ ...p, api_type: e.target.value })}>
                  {availableTypes.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
              </div>
              <div className="form-field">
                <label>Base URL</label>
                <input value={p.base_url} onChange={(e) => set({ ...p, base_url: e.target.value })} placeholder="https://api.openai.com/v1" />
              </div>
              <div className="form-field">
                <label>Health Endpoint</label>
                <input value={p.health_endpoint ?? ''} onChange={(e) => set({ ...p, health_endpoint: e.target.value })} />
              </div>
              <div className="form-field">
                <label>API Keys (comma separated)</label>
                <input value={p.api_keys?.join(', ') ?? ''} onChange={(e) => set({ ...p, api_keys: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} />
              </div>
              <div className="form-field">
                <label>Auth Scheme</label>
                <input value={p.auth_scheme ?? ''} onChange={(e) => set({ ...p, auth_scheme: e.target.value })} placeholder="Bearer" />
              </div>
              <div className="form-field">
                <label>Auth Header</label>
                <input value={p.auth_header ?? ''} onChange={(e) => set({ ...p, auth_header: e.target.value })} placeholder="Authorization" />
              </div>
              <div className="form-field">
                <label>Timeout</label>
                <input value={p.timeout ?? ''} onChange={(e) => set({ ...p, timeout: e.target.value })} placeholder="30s" />
              </div>
              <div className="form-field">
                <label>Priority</label>
                <input type="number" value={p.priority ?? 0} onChange={(e) => set({ ...p, priority: Number(e.target.value) })} />
              </div>
            </>
          )}
          {kind === 'route' && (
            <>
              <div className="form-field">
                <label>Model</label>
                <input value={r.model} onChange={(e) => setR({ ...r, model: e.target.value })} placeholder="gpt-4o" />
              </div>
              <div className="form-field">
                <label>Tenant</label>
                <input value={r.tenant} onChange={(e) => setR({ ...r, tenant: e.target.value })} placeholder="default" />
              </div>
              <div className="form-field">
                <label>Providers (comma separated)</label>
                <input value={r.providers.join(', ')} onChange={(e) => setR({ ...r, providers: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} placeholder="openai, fallback" />
              </div>
            </>
          )}
          {kind === 'rate' && (
            <>
              <div className="form-field">
                <label>Model</label>
                <input value={c.model} onChange={(e) => setC({ ...c, model: e.target.value })} placeholder="gpt-4o" />
              </div>
              <div className="form-field">
                <label>Input Price / 1K</label>
                <input type="number" step="any" value={c.input_price_per_1k} onChange={(e) => setC({ ...c, input_price_per_1k: Number(e.target.value) })} />
              </div>
              <div className="form-field">
                <label>Output Price / 1K</label>
                <input type="number" step="any" value={c.output_price_per_1k} onChange={(e) => setC({ ...c, output_price_per_1k: Number(e.target.value) })} />
              </div>
              <div className="form-field">
                <label>Currency</label>
                <input value={c.currency} onChange={(e) => setC({ ...c, currency: e.target.value.toUpperCase() })} placeholder="USD" />
              </div>
            </>
          )}
        </div>
        <div className="form-actions">
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Save</Button>
        </div>
      </div>
    </div>
  )
}