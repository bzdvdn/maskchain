import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { AsyncSection, Button, Modal, StatusPill } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/Toast'
import {
  GLOBAL_TENANT,
  deleteCostRate,
  deleteRoute,
  listModels,
  listProviders,
  upsertCostRate,
  upsertRoute,
  type ModelAggregate,
  type ProviderDto,
} from '../api/routing'

// @sk-task routing-ia#T3.1: Models page — cost and default providers (AC-006)
export function Models() {
  const { toast } = useToast()
  const [models, setModels] = useState<ModelAggregate[]>([])
  const [providers, setProviders] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)
  const [editing, setEditing] = useState<ModelAggregate | null>(null)
  const [deleting, setDeleting] = useState<ModelAggregate | null>(null)
  const [busy, setBusy] = useState(false)

  const reload = async () => {
    setLoading(true)
    setError(null)
    try {
      const [m, p] = await Promise.all([listModels(), listProviders()])
      setModels(m ?? [])
      setProviders((p ?? []).map((x: ProviderDto) => x.name).sort())
    } catch (err) {
      setError(err)
      toast('Failed to load models', 'error')
    } finally {
      setLoading(false)
    }
  }

  useAsyncData(async () => {
    await reload()
    return null
  }, [])

  const rows = useMemo(() => [...models].sort((a, b) => (a.model < b.model ? -1 : 1)), [models])

  const doDelete = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      await deleteCostRate(deleting.model)
      await deleteRoute({ tenant: GLOBAL_TENANT, model: deleting.model, providers: [] })
      toast(`Model "${deleting.model}" deleted`, 'success')
      setDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Delete failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  const handleSave = async (payload: ModelAggregate) => {
    try {
      await upsertCostRate({
        model: payload.model,
        input_price_per_1k: payload.input_price_per_1k,
        output_price_per_1k: payload.output_price_per_1k,
        currency: payload.currency || 'USD',
      })
      await upsertRoute({ tenant: GLOBAL_TENANT, model: payload.model, providers: payload.default_providers ?? [] })
      toast('Model saved', 'success')
      setEditing(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Save failed', 'error')
    }
  }

  return (
    <div>
      <div className="card table-card">
        <div className="card-header-row">
          <h3>Models</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setEditing({ model: '', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD', default_providers: [], override_count: 0 })}>Add Model</Button>
          </div>
        </div>
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr>
                <th>Model</th>
                <th className="num">Input / 1K</th>
                <th className="num">Output / 1K</th>
                <th>Currency</th>
                <th>Default providers (fallback order)</th>
                <th className="num">Overrides</th>
                <th className="num">Actions</th>
              </tr>
            </thead>
            <AsyncSection
              as="tbody"
              colSpan={7}
              loading={loading}
              error={error}
              onRetry={reload}
              empty={rows.length === 0}
              emptyMessage="No models yet"
              emptyAction={<Button size="small" onClick={() => setEditing({ model: '', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD', default_providers: [], override_count: 0 })}>Add Model</Button>}
            >
              {rows.map((m) => (
                <tr key={m.model}>
                  <td className="mono">{m.model}</td>
                  <td className="num">{m.input_price_per_1k}</td>
                  <td className="num">{m.output_price_per_1k}</td>
                  <td>{m.currency || 'USD'}</td>
                  <td>
                    {(m.default_providers ?? []).length === 0 ? (
                      <span className="muted">none — not routed</span>
                    ) : (
                      m.default_providers.map((p, i) => (
                        <span key={p}>
                          <span className="chip">{p}</span>
                          {i === m.default_providers.length - 1 ? '' : <span className="mono route-arr"> → </span>}
                        </span>
                      ))
                    )}
                  </td>
                  <td className="num">
                    {m.override_count > 0 ? <StatusPill tone="blue">{m.override_count}</StatusPill> : <span className="muted">0</span>}
                  </td>
                  <td>
                    <div className="u-actions">
                      <Button size="small" onClick={() => setEditing(m)}>Edit</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting(m)}>Delete</Button>
                    </div>
                  </td>
                </tr>
              ))}
            </AsyncSection>
          </table>
        </div>
        <div className="muted meta-sm" style={{ padding: '8px 12px' }}>
          Default providers apply to every tenant that has no override; overrides are managed on the Routing page.
        </div>
      </div>

      {editing && (
        <ModelModal
          initial={editing}
          providers={providers}
          onClose={() => setEditing(null)}
          onSave={handleSave}
        />
      )}

      <ConfirmModal
        open={!!deleting}
        title="Delete model"
        message={`Delete "${deleting?.model}"? Its cost rate and default route are removed; tenant overrides are not.`}
        busy={busy}
        onConfirm={doDelete}
        onCancel={() => setDeleting(null)}
      />
    </div>
  )
}

function ModelModal({
  initial,
  providers,
  onClose,
  onSave,
}: {
  initial: ModelAggregate
  providers: string[]
  onClose: () => void
  onSave: (m: ModelAggregate) => void
}) {
  const [m, setM] = useState<ModelAggregate>({ ...initial, default_providers: initial.default_providers ?? [] })
  const [err, setErr] = useState('')
  const isNew = !initial.model
  const set = (patch: Partial<ModelAggregate>) => setM((prev) => ({ ...prev, ...patch }))

  const selected = m.default_providers ?? []
  const toggleProvider = (name: string) => {
    set({ default_providers: selected.includes(name) ? selected.filter((p) => p !== name) : [...selected, name] })
  }

  const submit = () => {
    setErr('')
    if (!m.model) { setErr('Model id is required'); return }
    if (m.input_price_per_1k < 0 || m.output_price_per_1k < 0) { setErr('Prices must not be negative'); return }
    onSave({ ...m, currency: (m.currency || 'USD').toUpperCase() })
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={isNew ? 'Add Model' : 'Edit Model'}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Save</Button>
        </>
      }
    >
      {err && <div className="confirm-dialog u-mt8"><p>{err}</p></div>}
          <div className="form-field">
            <label>Model id</label>
            <input value={m.model} onChange={(e) => set({ model: e.target.value })} placeholder="openai/gpt-4o-mini" disabled={!isNew} />
          </div>
          <div className="form-field"><label>Input price / 1K</label><input type="number" step="any" value={m.input_price_per_1k} onChange={(e) => set({ input_price_per_1k: Number(e.target.value) })} /></div>
          <div className="form-field"><label>Output price / 1K</label><input type="number" step="any" value={m.output_price_per_1k} onChange={(e) => set({ output_price_per_1k: Number(e.target.value) })} /></div>
          <div className="form-field"><label>Currency</label><input value={m.currency ?? 'USD'} onChange={(e) => set({ currency: e.target.value.toUpperCase() })} placeholder="USD" /></div>
          <div className="form-field">
            <label>Default providers (click in fallback order; first = primary)</label>
            <div className="u-wrap u-mb8">
              {providers.length === 0 && <span className="muted">No providers yet — add one on the Providers page.</span>}
              {providers.map((name) => (
                <Button key={name} size="small" variant={selected.includes(name) ? 'primary' : 'default'} onClick={() => toggleProvider(name)}>
                  {selected.includes(name) ? `${selected.indexOf(name) + 1}. ` : ''}{name}
                </Button>
              ))}
            </div>
            {selected.length === 0 && <div className="muted meta-sm">No default providers: this model routes nowhere until configured.</div>}
          </div>
          {!isNew && <div className="muted meta-sm">{m.override_count} tenant override{m.override_count === 1 ? '' : 's'} — edit on the Routing page.</div>}
    </Modal>
  )
}
