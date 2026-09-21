import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Button, ChipInput, EmptyState, StatusPill, type StatusTone } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/Toast'
import { relativeTime } from '../utils/format'
import {
  deleteProvider,
  isMaskedKey,
  listModels,
  listProviderModels,
  listProviders,
  upsertProvider,
  type ModelAggregate,
  type ProviderDto,
} from '../api/routing'

const API_TYPES = ['openai', 'anthropic', 'ollama', 'proxy', 'gemini', 'bedrock']

function toneForStatus(status?: string): StatusTone {
  if (status === 'up') return 'green'
  if (status === 'down') return 'red'
  if (status === 'degraded') return 'amber'
  return 'gray'
}

// @sk-task routing-ia#T2.4: Providers page — credentials, proxy, models (AC-005)
export function Providers() {
  const { toast } = useToast()
  const [providers, setProviders] = useState<ProviderDto[]>([])
  const [models, setModels] = useState<ModelAggregate[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState<ProviderDto | null>(null)
  const [deleting, setDeleting] = useState<ProviderDto | null>(null)
  const [busy, setBusy] = useState(false)

  const reload = async () => {
    setLoading(true)
    try {
      const [p, m] = await Promise.all([listProviders(), listModels()])
      setProviders(p ?? [])
      setModels(m ?? [])
    } catch {
      toast('Failed to load providers', 'error')
    } finally {
      setLoading(false)
    }
  }

  useAsyncData(async () => {
    await reload()
    return null
  }, [])

  const cards = useMemo(() => [...providers].sort((a, b) => (a.name < b.name ? -1 : 1)), [providers])
  const modelsOf = (name: string) => models.filter((m) => (m.default_providers ?? []).includes(name)).map((m) => m.model)

  const doDelete = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      await deleteProvider(deleting.name)
      toast(`Provider "${deleting.name}" deleted`, 'success')
      setDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Delete failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  const handleSave = async (payload: ProviderDto) => {
    try {
      await upsertProvider(payload)
      toast('Provider saved', 'success')
      setEditing(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Save failed', 'error')
    }
  }

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Providers</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditing({ name: '', base_url: '', api_type: 'openai', api_keys: [], models: [] })}>Add Provider</Button>
          </div>
        </div>
        {!loading && cards.length === 0 ? (
          <EmptyState
            message="No providers configured"
            action={<Button size="small" onClick={() => setEditing({ name: '', base_url: '', api_type: 'openai', api_keys: [], models: [] })}>Add Provider</Button>}
          />
        ) : (
          <div className="providers">
            {cards.map((p) => {
              const attached = modelsOf(p.name)
              return (
                <div key={p.name} className="provider">
                  <div className="card-header-row u-mb10">
                    <div>
                      <div className="name">{p.name}</div>
                      <div className="sub muted">{p.api_type} · <code>{p.base_url}</code></div>
                    </div>
                    <StatusPill tone={toneForStatus(p.status)}>{p.status ?? 'unknown'}</StatusPill>
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
                      <div className="num m-v">{attached.length}</div>
                      <div className="muted m-k">models</div>
                    </div>
                  </div>
                  <div className="u-wrap u-mb10">
                    {attached.slice(0, 3).map((m) => <span key={m} className="chip">{m}</span>)}
                    {attached.length > 3 && <span className="chip">+{attached.length - 3}</span>}
                    {attached.length === 0 && <span className="muted">no models attached</span>}
                  </div>
                  {p.proxy_url && <div className="muted meta-sm u-mb10">proxy: <code>{p.proxy_url}</code></div>}
                  <div className="row-actions">
                    <Button size="small" onClick={() => setEditing({ ...p, models: attached })}>Edit</Button>
                    <Button size="small" variant="danger" onClick={() => setDeleting(p)}>Delete</Button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {editing && (
        <ProviderModal
          initial={editing}
          onClose={() => setEditing(null)}
          onSave={handleSave}
        />
      )}

      <ConfirmModal
        open={!!deleting}
        title="Delete provider"
        message={`Delete "${deleting?.name}"? Routes referencing it will skip the missing provider.`}
        busy={busy}
        onConfirm={doDelete}
        onCancel={() => setDeleting(null)}
      />
    </div>
  )
}

function ProviderModal({
  initial,
  onClose,
  onSave,
}: {
  initial: ProviderDto
  onClose: () => void
  onSave: (p: ProviderDto) => void
}) {
  const [p, setP] = useState<ProviderDto>({ ...initial, api_keys: initial.api_keys ?? [], models: initial.models ?? [] })
  const [err, setErr] = useState('')
  const [available, setAvailable] = useState<string[] | null>(null)
  const [loadingModels, setLoadingModels] = useState(false)
  const [modelQuery, setModelQuery] = useState('')
  const [modelErr, setModelErr] = useState('')
  const isNew = !initial.name
  const set = (patch: Partial<ProviderDto>) => setP((prev) => ({ ...prev, ...patch }))

  const selected = p.models ?? []
  const toggleModel = (id: string) => {
    set({ models: selected.includes(id) ? selected.filter((m) => m !== id) : [...selected, id] })
  }

  async function loadModels() {
    if (!p.name) {
      setModelErr('Set the provider name first, then load models.')
      return
    }
    setLoadingModels(true)
    setModelErr('')
    try {
      const ids = await listProviderModels(p.name)
      setAvailable(ids)
      if (ids.length === 0) setModelErr('The provider returned no models — add them manually below.')
    } catch (e: any) {
      const detail = e?.body?.error?.message ?? e?.message ?? 'unknown error'
      setAvailable(null)
      setModelErr(`Could not load models (${detail}) — add them manually below.`)
    } finally {
      setLoadingModels(false)
    }
  }

  const filtered = (available ?? []).filter((m) => m.toLowerCase().includes(modelQuery.trim().toLowerCase()))

  const submit = () => {
    setErr('')
    if (!p.name || !p.base_url) { setErr('Name and Base URL are required'); return }
    const keys = (p.api_keys ?? []).filter(Boolean)
    const hasKey = keys.length > 0
    const hasAWSCreds = !!(p.aws_access_key_id && p.aws_secret_access_key)
    if (p.api_type === 'bedrock') {
      if (!hasAWSCreds && !hasKey) { setErr('Bedrock requires AWS credentials (region, access key, secret) or an API key'); return }
    } else if (p.api_type !== 'ollama' && !hasKey) {
      setErr('Add at least one API key')
      return
    }
    onSave({ ...p, api_keys: keys })
  }

  const maskedNote = (initial.api_keys ?? []).some(isMaskedKey)

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal generic-modal" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <h3>{isNew ? 'Add Provider' : 'Edit Provider'}</h3>
        {err && <div className="confirm-dialog u-mt8"><p>{err}</p></div>}
        <div className="modal-body">
          <div className="form-field"><label>Name</label><input value={p.name ?? ''} onChange={(e) => set({ name: e.target.value })} placeholder="openrouter" /></div>
          <div className="form-field">
            <label>API Type</label>
            <select value={p.api_type ?? 'openai'} onChange={(e) => set({ api_type: e.target.value })}>
              {API_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
            </select>
          </div>
          <div className="form-field"><label>Base URL</label><input value={p.base_url ?? ''} onChange={(e) => set({ base_url: e.target.value })} placeholder="https://openrouter.ai/api/v1" /></div>
          <div className="form-field"><label>Health Endpoint</label><input value={p.health_endpoint ?? ''} onChange={(e) => set({ health_endpoint: e.target.value })} placeholder="/models" /></div>
          <div className="form-field">
            <label>API Keys (comma separated)</label>
            <input value={(p.api_keys ?? []).join(', ')} onChange={(e) => set({ api_keys: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} />
            {maskedNote && <div className="muted meta-sm u-mt4">Existing key is masked — leave it as is to keep it unchanged.</div>}
          </div>
          <div className="form-field"><label>Auth Scheme</label><input value={p.auth_scheme ?? ''} onChange={(e) => set({ auth_scheme: e.target.value })} placeholder="bearer | api-key | basic" /></div>
          <div className="form-field"><label>Auth Header</label><input value={p.auth_header ?? ''} onChange={(e) => set({ auth_header: e.target.value })} placeholder="Authorization" /></div>
          <div className="form-field"><label>Auth Prefix</label><input value={p.auth_prefix ?? ''} onChange={(e) => set({ auth_prefix: e.target.value })} placeholder="Bearer " /></div>
          <div className="form-field">
            <label>Egress proxy URL (optional)</label>
            <input value={p.proxy_url ?? ''} onChange={(e) => set({ proxy_url: e.target.value })} placeholder="http://corp-proxy:3128 or socks5://proxy:1080" />
            <div className="muted meta-sm u-mt4">Provider requests egress through this proxy. Applies to HTTP APIs (not Bedrock).</div>
          </div>
          <div className="form-field"><label>Timeout</label><input value={p.timeout ?? ''} onChange={(e) => set({ timeout: e.target.value })} placeholder="60s" /></div>
          <div className="form-field"><label>Priority</label><input type="number" value={p.priority ?? 0} onChange={(e) => set({ priority: Number(e.target.value) })} /></div>

          {p.api_type === 'bedrock' && (
            <>
              <div className="form-field"><label>AWS Region</label><input value={p.aws_region ?? ''} onChange={(e) => set({ aws_region: e.target.value })} placeholder="us-east-1" /></div>
              <div className="form-field"><label>AWS Access Key ID</label><input value={p.aws_access_key_id ?? ''} onChange={(e) => set({ aws_access_key_id: e.target.value })} /></div>
              <div className="form-field"><label>AWS Secret Access Key</label><input type="password" value={p.aws_secret_access_key ?? ''} onChange={(e) => set({ aws_secret_access_key: e.target.value })} /></div>
            </>
          )}

          <div className="form-field">
            <label>Models served by this provider (global default)</label>
            <div className="u-flex u-mb8" style={{ gap: 8, alignItems: 'center' }}>
              <Button size="small" onClick={loadModels} disabled={loadingModels}>
                {loadingModels ? 'Loading…' : 'Load models from provider'}
              </Button>
              {selected.length > 0 && <span className="muted meta-sm">{selected.length} selected</span>}
            </div>
            {modelErr && <div className="muted meta-sm u-mb6">{modelErr}</div>}
            {available && available.length > 0 && (
              <>
                <input
                  value={modelQuery}
                  onChange={(e) => setModelQuery(e.target.value)}
                  placeholder="Search models…"
                  style={{ marginBottom: 6 }}
                />
                <div style={{ maxHeight: 200, overflowY: 'auto', border: '1px solid var(--border)', borderRadius: 6, padding: 6 }}>
                  {filtered.slice(0, 300).map((id) => (
                    <label
                      key={id}
                      style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '3px 2px', cursor: 'pointer', lineHeight: 1.4 }}
                    >
                      <input
                        type="checkbox"
                        checked={selected.includes(id)}
                        onChange={() => toggleModel(id)}
                        style={{ margin: 0, flexShrink: 0, width: 14, height: 14 }}
                      />
                      <span className="mono">{id}</span>
                    </label>
                  ))}
                  {filtered.length === 0 && <div className="muted meta-sm">No match</div>}
                </div>
              </>
            )}
            <div className="u-mt8">
              <label className="muted meta-sm">Selected models (editable manually)</label>
              <ChipInput value={selected} onChange={(models) => set({ models })} placeholder="press Enter to add a model id manually" />
            </div>
            <div className="muted meta-sm u-mt4">Attached models get this provider in their default provider chain; tenant overrides on the Routing page still win.</div>
          </div>
        </div>
        <div className="form-actions">
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Save</Button>
        </div>
      </div>
    </div>
  )
}
