import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { useWorkspace } from '../hooks/useWorkspace'
import { useAutoRefresh } from '../hooks/useAutoRefresh'
import { AsyncSection, Button, Modal, StatusPill } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/Toast'
import { Link } from 'react-router-dom'
import {
  GLOBAL_TENANT,
  deleteAlias,
  deleteRoute,
  listAliases,
  listModels,
  listProviders,
  listRoutes,
  upsertAlias,
  upsertRoute,
  type AliasDto,
  type ModelAggregate,
  type RouteDto,
} from '../api/routing'

// @sk-task routing-ia#T3.2: Routing page — per-tenant overrides (AC-007)
export function Routing() {
  const { toast } = useToast()
  const [routes, setRoutes] = useState<RouteDto[]>([])
  const [models, setModels] = useState<ModelAggregate[]>([])
  const [providers, setProviders] = useState<string[]>([])
  const [aliases, setAliases] = useState<AliasDto[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [editing, setEditing] = useState<RouteDto | null>(null)
  const [deleting, setDeleting] = useState<RouteDto | null>(null)
  const [aliasEditing, setAliasEditing] = useState<AliasDto | null>(null)
  const [aliasDeleting, setAliasDeleting] = useState<AliasDto | null>(null)
  const [busy, setBusy] = useState(false)

  const reload = async () => {
    setLoading(true)
    setError(null)
    try {
      const [r, m, p, a] = await Promise.all([listRoutes(), listModels(), listProviders(), listAliases()])
      setRoutes(r ?? [])
      setModels(m ?? [])
      setProviders((p ?? []).map((x) => x.name).sort())
      setAliases(a ?? [])
    } catch (err) {
      setError(err)
      toast('Failed to load routing', 'error')
    } finally {
      setLoading(false)
    }
  }

  useAsyncData(async () => {
    await reload()
    return null
  }, [])

  useAutoRefresh(reload, 30)

  const [workspace] = useWorkspace()
  const overrides = useMemo(
    () => routes.filter((r) => r.tenant !== GLOBAL_TENANT && (workspace === '' || r.tenant === workspace)),
    [routes, workspace],
  )
  const inheriting = useMemo(
    () => models.filter((m) => m.override_count === 0 && (m.default_providers ?? []).length > 0),
    [models],
  )

  const tenantLabel = (t: string) => (t === '' ? 'default' : t)

  const doDelete = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      await deleteRoute(deleting)
      toast(`Override for ${tenantLabel(deleting.tenant)}/${deleting.model} deleted`, 'success')
      setDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Delete failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  const handleSave = async (payload: RouteDto) => {
    try {
      await upsertRoute(payload)
      toast('Override saved', 'success')
      setEditing(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Save failed', 'error')
    }
  }

  // @sk-task model-aliases-weighted-lb#T3.4: tenant alias management (AC-009)
  const saveAlias = async (payload: AliasDto) => {
    try {
      await upsertAlias(payload)
      toast('Alias saved', 'success')
      setAliasEditing(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Save failed', 'error')
    }
  }

  const removeAlias = async () => {
    if (!aliasDeleting) return
    setBusy(true)
    try {
      await deleteAlias(aliasDeleting)
      toast(`Alias "${aliasDeleting.alias}" deleted`, 'success')
      setAliasDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Delete failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <div className="card table-card">
        <div className="card-header-row">
          <h3>Tenant overrides</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditing({ tenant: 'default', model: '', providers: [] })}>Add Override</Button>
          </div>
        </div>
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr><th>Tenant</th><th>Model</th><th>Providers (fallback order)</th><th className="num">Actions</th></tr>
            </thead>
            <AsyncSection
              as="tbody"
              colSpan={4}
              loading={loading}
              error={error}
              onRetry={reload}
              empty={overrides.length === 0}
              emptyMessage="No tenant overrides — every tenant uses the default providers from Models"
              emptyAction={<Button size="small" onClick={() => setEditing({ tenant: 'default', model: '', providers: [] })}>Add Override</Button>}
            >
              {overrides.map((r) => (
                <tr key={`${r.tenant}/${r.model}`}>
                  <td><code>{tenantLabel(r.tenant)}</code></td>
                  <td className="mono">{r.model}</td>
                  <td>
                    {r.providers.map((p, i) => (
                      <span key={p}>
                        <span className="chip">{p}</span>
                        {i === r.providers.length - 1 ? '' : <span className="mono route-arr"> → </span>}
                      </span>
                    ))}
                  </td>
                  <td>
                    <div className="u-actions">
                      <Button size="small" onClick={() => setEditing(r)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting(r)}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
            </AsyncSection>
          </table>
        </div>
        <div className="muted meta-sm" style={{ padding: '8px 12px' }}>
          An override replaces the model's default providers for that tenant. Defaults are managed on the <Link className="btn-link" to="/models">Models</Link> page.
        </div>
      </div>

      <div className="card table-card u-mt16">
        <div className="card-header-row"><h3>Inheriting the global default</h3></div>
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr><th>Model</th><th>Default providers (fallback order)</th><th className="num">Overrides</th></tr>
            </thead>
            <AsyncSection
              as="tbody"
              colSpan={3}
              loading={loading}
              empty={inheriting.length === 0}
              emptyMessage="No models with a global default"
            >
              {inheriting.map((m) => (
                <tr key={m.model}>
                  <td className="mono">{m.model}</td>
                  <td>
                    {m.default_providers.map((p, i) => (
                      <span key={p}>
                        <span className="chip">{p}</span>
                        {i === m.default_providers.length - 1 ? '' : <span className="mono route-arr"> → </span>}
                      </span>
                    ))}
                  </td>
                  <td className="num"><StatusPill tone="gray" withDot={false}>inherited</StatusPill></td>
                </tr>
              ))}
            </AsyncSection>
          </table>
        </div>
      </div>

      <div className="card table-card u-mt16">
        <div className="card-header-row">
          <h3>Model aliases</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setAliasEditing({ tenant: 'default', alias: '', target: '' })}>Add Alias</Button>
          </div>
        </div>
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr><th>Tenant</th><th>Alias</th><th>Routes to</th><th className="num">Actions</th></tr>
            </thead>
            <AsyncSection
              as="tbody"
              colSpan={4}
              loading={loading}
              empty={aliases.length === 0}
              emptyMessage="No aliases — requested models route by their own name"
              emptyAction={<Button size="small" onClick={() => setAliasEditing({ tenant: 'default', alias: '', target: '' })}>Add Alias</Button>}
            >
              {aliases.map((a) => (
                <tr key={`${a.tenant}/${a.alias}`}>
                  <td><code>{a.tenant === GLOBAL_TENANT ? 'all tenants' : tenantLabel(a.tenant)}</code></td>
                  <td className="mono">{a.alias}</td>
                  <td className="mono">{a.target}</td>
                  <td>
                    <div className="u-actions">
                      <Button size="small" onClick={() => setAliasEditing(a)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setAliasDeleting(a)}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
            </AsyncSection>
          </table>
        </div>
        <div className="muted meta-sm" style={{ padding: '8px 12px' }}>
          An alias maps a requested model name to the model that is actually routed, per tenant.
        </div>
      </div>

      {aliasEditing && (
        <AliasModal initial={aliasEditing} onClose={() => setAliasEditing(null)} onSave={saveAlias} />
      )}

      <ConfirmModal
        open={!!aliasDeleting}
        title="Delete alias"
        message={`Delete alias "${aliasDeleting?.alias}"? Requests will route by their own name.`}
        busy={busy}
        onConfirm={removeAlias}
        onCancel={() => setAliasDeleting(null)}
      />

      {editing && (
        <OverrideModal
          initial={editing}
          providers={providers}
          models={models.map((m) => m.model)}
          onClose={() => setEditing(null)}
          onSave={handleSave}
        />
      )}

      <ConfirmModal
        open={!!deleting}
        title="Delete override"
        message={`Delete the override for ${tenantLabel(deleting?.tenant ?? '')}/${deleting?.model}? That tenant will fall back to the model default.`}
        busy={busy}
        onConfirm={doDelete}
        onCancel={() => setDeleting(null)}
      />
    </div>
  )
}

function OverrideModal({
  initial,
  providers,
  models,
  onClose,
  onSave,
}: {
  initial: RouteDto
  providers: string[]
  models: string[]
  onClose: () => void
  onSave: (r: RouteDto) => void
}) {
  const [r, setR] = useState<RouteDto>({ ...initial, providers: initial.providers ?? [] })
  const [err, setErr] = useState('')
  const isNew = !initial.model
  const set = (patch: Partial<RouteDto>) => setR((prev) => ({ ...prev, ...patch }))

  const selected = r.providers ?? []
  const toggleProvider = (name: string) => {
    set({ providers: selected.includes(name) ? selected.filter((p) => p !== name) : [...selected, name] })
  }

  const submit = () => {
    setErr('')
    if (!r.tenant) { setErr('Tenant is required (use "default" for the default tenant)'); return }
    if (r.tenant === GLOBAL_TENANT) { setErr(`"${GLOBAL_TENANT}" is reserved for the global default — use the Models page`); return }
    if (!r.model) { setErr('Model is required'); return }
    if (selected.length === 0) { setErr('Select at least one provider'); return }
    onSave({ ...r, tenant: r.tenant === 'default' ? '' : r.tenant })
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={isNew ? 'Add Override' : 'Edit Override'}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Save</Button>
        </>
      }
    >
      {err && <div className="confirm-dialog u-mt8"><p>{err}</p></div>}
          <div className="form-field">
            <label>Tenant</label>
            <input value={r.tenant === '' ? 'default' : r.tenant} onChange={(e) => set({ tenant: e.target.value })} placeholder="default" disabled={!isNew} />
          </div>
          <div className="form-field">
            <label>Model</label>
            <input value={r.model} onChange={(e) => set({ model: e.target.value })} placeholder="openai/gpt-4o-mini" list="model-options" disabled={!isNew} />
            <datalist id="model-options">
              {models.map((m) => <option key={m} value={m} />)}
            </datalist>
          </div>
          <div className="form-field">
            <label>Providers (click in fallback order; first = primary)</label>
            <div className="u-wrap u-mb8">
              {providers.length === 0 && <span className="muted">No providers yet — add one on the Providers page.</span>}
              {providers.map((name) => (
                <Button key={name} size="small" variant={selected.includes(name) ? 'primary' : 'default'} onClick={() => toggleProvider(name)}>
                  {selected.includes(name) ? `${selected.indexOf(name) + 1}. ` : ''}{name}
                </Button>
              ))}
            </div>
          </div>
    </Modal>
  )
}

// @sk-task model-aliases-weighted-lb#T3.4: alias editor (AC-009)
function AliasModal({
  initial,
  onClose,
  onSave,
}: {
  initial: AliasDto
  onClose: () => void
  onSave: (a: AliasDto) => void
}) {
  const [a, setA] = useState<AliasDto>({ ...initial })
  const [err, setErr] = useState('')
  const set = (patch: Partial<AliasDto>) => setA((prev) => ({ ...prev, ...patch }))

  const submit = () => {
    setErr('')
    if (!a.alias) { setErr('Alias is required'); return }
    if (!a.target) { setErr('Target model is required'); return }
    onSave({ ...a })
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={initial.alias ? 'Edit Alias' : 'Add Alias'}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Save</Button>
        </>
      }
    >
      {err && <div className="confirm-dialog u-mt8"><p>{err}</p></div>}
      <div className="form-field"><label>Tenant</label><input value={a.tenant === '' ? 'default' : a.tenant} onChange={(e) => set({ tenant: e.target.value })} placeholder="default" /></div>
      <div className="form-field"><label>Alias (requested model)</label><input value={a.alias} onChange={(e) => set({ alias: e.target.value })} placeholder="gpt-4o" /></div>
      <div className="form-field"><label>Routes to (target model)</label><input value={a.target} onChange={(e) => set({ target: e.target.value })} placeholder="openai/gpt-4o-2024" /></div>
      <div className="muted meta-sm">An alias is single-hop: the target is routed as-is (exact or wildcard).</div>
    </Modal>
  )
}
