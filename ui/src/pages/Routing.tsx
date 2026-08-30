import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Button, EmptyState, StatusDot, StatusPill, type StatusTone } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/Toast'
import { relativeTime } from '../utils/format'
import {
  deleteCostRate,
  deleteProvider,
  deleteRoute,
  isMaskedKey,
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

function toneForStatus(status?: string): StatusTone {
  if (status === 'up') return 'green'
  if (status === 'down') return 'red'
  if (status === 'degraded') return 'amber'
  return 'gray'
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

  const statusOf = (name: string) => providers.find((p) => p.name === name)?.status

  const cards = useMemo(() => [...providers].sort((a, b) => (a.name < b.name ? -1 : 1)), [providers])

  const [editingProvider, setEditingProvider] = useState<ProviderDto | null>(null)
  const [editingRoute, setEditingRoute] = useState<RouteDto | null>(null)
  const [editingRate, setEditingRate] = useState<CostRateDto | null>(null)
  const [deleting, setDeleting] = useState<null | { kind: 'provider' | 'route' | 'rate'; name: string; payload?: unknown }>(null)
  const [busy, setBusy] = useState(false)

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
      <div className="card u-mb16">
        <div className="card-header-row">
          <h3>Providers</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditingProvider({} as ProviderDto)}>Add Provider</Button>
          </div>
        </div>
        {!loading && cards.length === 0 ? (
          <EmptyState message="No providers configured" action={<Button size="small" onClick={() => setEditingProvider({} as ProviderDto)}>Add Provider</Button>} />
        ) : (
          <div className="providers">
            {cards.map((p) => {
              const tone = toneForStatus(p.status)
              const routes = rules.filter((r) => (r.providers ?? []).includes(p.name))
              const fallbackRules = rules.filter((r) => {
                const idx = (r.providers ?? []).indexOf(p.name)
                return idx > 0
              })
              return (
                <div key={p.name} className="provider">
                  <div className="card-header-row u-mb10">
                    <div>
                      <div className="name">{p.name}</div>
                      <div className="sub muted">{p.api_type} · <code>{p.base_url}</code></div>
                    </div>
                    <StatusPill tone={tone}>{p.status ?? 'unknown'}</StatusPill>
                  </div>
                  <div className="metric-row provider-metrics">
                    <div>
                      <div className="num m-v">{p.latency_ms != null ? `${p.latency_ms}ms` : '—'}</div>
                      <div className="muted m-k">latency</div>
                    </div>
                    <div>
                      <div className="num m-v">{p.last_check ? relativeTime(new Date(p.last_check * 1000).toISOString()) : '—'}</div>
                      <div className="muted m-k">last check</div>
                    </div>
                    <div>
                      <div className="num m-v">{routes.length}</div>
                      <div className="muted m-k">routes</div>
                    </div>
                  </div>
                  <div className="u-wrap u-mb10">
                    {routes.slice(0, 3).map((r) => (
                      <span key={`${r.tenant}/${r.model}`} className="chip">{r.model}@{r.tenant || 'default'}</span>
                    ))}
                    {routes.length > 3 && <span className="chip">+{routes.length - 3}</span>}
                    {routes.length === 0 && <span className="muted">no active routes</span>}
                  </div>
                  {fallbackRules.length > 0 && (
                    <div className="muted meta-sm u-mb10">
                      used as fallback for {fallbackRules.length} rule{fallbackRules.length > 1 ? 's' : ''}
                    </div>
                  )}
                  <div className="row-actions">
                    <Button size="small" onClick={() => setEditingProvider(p)}>Edit</Button>
                    <Button size="small" variant="danger" onClick={() => setDeleting({ kind: 'provider', name: p.name })}>Delete</Button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <div className="card table-card">
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr><th>Model</th><th>Tenant</th><th>Providers</th><th>Health</th><th>Source</th><th className="num">Actions</th></tr>
            </thead>
            <tbody>
              {rules.map((r) => {
                const first = (r.providers ?? [])[0]
                const health = statusOf(first)
                return (
                  <tr key={`${r.tenant}/${r.model}`}>
                    <td className="mono">{r.model}</td>
                    <td>{r.tenant || 'default'}</td>
                    <td>
                      {r.providers.map((p, i) => (
                        <span key={p}>
                          <span className="chip">{p}</span>
                          {i === r.providers.length - 1 ? '' : <span className="mono route-arr"> → </span>}
                        </span>
                      ))}
                    </td>
                    <td><StatusDot tone={toneForStatus(health)} title={health ?? 'unknown'} /></td>
                    <td><span className="muted">{r.source ?? '—'}</span></td>
                    <td>
                      <div className="u-actions">
                        <Button size="small" onClick={() => setEditingRoute(r)}>Edit</Button>
                        <Button size="small" variant="danger" onClick={() => setDeleting({ kind: 'route', name: `${r.tenant}/${r.model}`, payload: r })}>Delete</Button>
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!loading && rules.length === 0 && <tr><td colSpan={6}><EmptyState message="No routing rules" /></td></tr>}
              {loading && <tr><td colSpan={6} className="tbl-progress">Loading routing rules…</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      <div className="card table-card u-mt16">
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr><th>Model</th><th className="num">Input / 1K</th><th className="num">Output / 1K</th><th>Currency</th><th>Source</th><th className="num">Actions</th></tr>
            </thead>
            <tbody>
              {rates.map((r) => (
                <tr key={r.model}>
                  <td className="mono">{r.model}</td>
                  <td className="num">{r.input_price_per_1k}</td>
                  <td className="num">{r.output_price_per_1k}</td>
                  <td>{r.currency || 'USD'}</td>
                  <td><span className="muted">{r.source ?? '—'}</span></td>
                  <td>
                    <div className="u-actions">
                      <Button size="small" onClick={() => setEditingRate(r)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting({ kind: 'rate', name: r.model })}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && rates.length === 0 && <tr><td colSpan={6}><EmptyState message="No cost rates configured" /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {(editingProvider || editingRoute || editingRate) && (
        <CrudModal
          key={`${editingProvider?.name ?? ''}-${editingRoute?.model ?? ''}-${editingRate?.model ?? ''}`}
          kind={editingProvider ? 'provider' : editingRoute ? 'route' : 'rate'}
          availableTypes={['openai', 'anthropic', 'ollama', 'proxy', 'gemini', 'bedrock']}
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
      const keys = p.api_keys ?? []
      const hasEditableKey = keys.some((k) => k && !isMaskedKey(k))
      const hasAWSCreds = (p.aws_access_key_id && !isMaskedKey(p.aws_access_key_id) && p.aws_secret_access_key && !isMaskedKey(p.aws_secret_access_key)) ?? false
      if (!hasEditableKey && !hasAWSCreds) { setErr('Add at least one API key or AWS access key'); return }
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
        {err && <div className="confirm-dialog u-mt8"><p>{err}</p></div>}
        <div className="modal-body">
          {kind === 'provider' && (
            <>
              <div className="form-field"><label>Name</label><input value={p.name} onChange={(e) => set({ ...p, name: e.target.value })} placeholder="my-provider" /></div>
              <div className="form-field"><label>API Type</label><select value={p.api_type} onChange={(e) => set({ ...p, api_type: e.target.value })}>{availableTypes.map((t) => <option key={t} value={t}>{t}</option>)}</select></div>
              <div className="form-field"><label>Base URL</label><input value={p.base_url} onChange={(e) => set({ ...p, base_url: e.target.value })} placeholder="https://api.openai.com/v1" /></div>
              <div className="form-field"><label>Health Endpoint</label><input value={p.health_endpoint ?? ''} onChange={(e) => set({ ...p, health_endpoint: e.target.value })} /></div>
              <div className="form-field"><label>API Keys (comma separated)</label><input value={p.api_keys?.join(', ') ?? ''} onChange={(e) => set({ ...p, api_keys: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} /></div>
              <div className="form-field"><label>Auth Scheme</label><input value={p.auth_scheme ?? ''} onChange={(e) => set({ ...p, auth_scheme: e.target.value })} placeholder="Bearer" /></div>
              <div className="form-field"><label>Auth Header</label><input value={p.auth_header ?? ''} onChange={(e) => set({ ...p, auth_header: e.target.value })} placeholder="Authorization" /></div>
              <div className="form-field"><label>Timeout</label><input value={p.timeout ?? ''} onChange={(e) => set({ ...p, timeout: e.target.value })} placeholder="30s" /></div>
              <div className="form-field"><label>Priority</label><input type="number" value={p.priority ?? 0} onChange={(e) => set({ ...p, priority: Number(e.target.value) })} /></div>
            </>
          )}
          {kind === 'route' && (
            <>
              <div className="form-field"><label>Model</label><input value={r.model} onChange={(e) => setR({ ...r, model: e.target.value })} placeholder="gpt-4o" /></div>
              <div className="form-field"><label>Tenant</label><input value={r.tenant} onChange={(e) => setR({ ...r, tenant: e.target.value })} placeholder="default" /></div>
              <div className="form-field"><label>Providers (comma separated, first = primary)</label><input value={r.providers.join(', ')} onChange={(e) => setR({ ...r, providers: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} placeholder="openai, fallback" /></div>
            </>
          )}
          {kind === 'rate' && (
            <>
              <div className="form-field"><label>Model</label><input value={c.model} onChange={(e) => setC({ ...c, model: e.target.value })} placeholder="gpt-4o" /></div>
              <div className="form-field"><label>Input Price / 1K</label><input type="number" step="any" value={c.input_price_per_1k} onChange={(e) => setC({ ...c, input_price_per_1k: Number(e.target.value) })} /></div>
              <div className="form-field"><label>Output Price / 1K</label><input type="number" step="any" value={c.output_price_per_1k} onChange={(e) => setC({ ...c, output_price_per_1k: Number(e.target.value) })} /></div>
              <div className="form-field"><label>Currency</label><input value={c.currency} onChange={(e) => setC({ ...c, currency: e.target.value.toUpperCase() })} placeholder="USD" /></div>
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