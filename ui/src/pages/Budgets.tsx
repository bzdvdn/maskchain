import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Badge, Button, EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { useToast } from '../components/Toast'
import { useSort, sortRows } from '../hooks/useSort'
import { budgetHistory, createBudget, deleteBudget, listBudgets, updateBudget, type BudgetDto, type CreateBudgetRequest, type SpendEntryDto } from '../api/budgets'
import { listTenants } from '../api/tenants'
import { listKeys } from '../api/keys'
import { useWorkspace } from '../hooks/useWorkspace'

function fmtTime(iso: string | undefined | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString()
}

function fmtMoney(v: number | undefined, currency: string | undefined): string {
  if (v === undefined || v === null) return '—'
  const cur = currency || 'USD'
  try {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency: cur }).format(v)
  } catch {
    return `${v} ${cur}`
  }
}

function scopeLabel(scope: string): string {
  switch (scope) {
    case 'key': return 'Key'
    case 'model': return 'Model'
    default: return 'Tenant'
  }
}

function periodLabel(b: BudgetDto): string {
  switch (b.type) {
    case 'monthly': return 'Monthly'
    case 'daily': return 'Daily'
    case 'custom': return `Every ${b.custom_days || 30}d`
    default: return b.type
  }
}

export function Budgets() {
  const { toast } = useToast()
  const [budgets, setBudgets] = useState<BudgetDto[]>([])
  const [tenants, setTenants] = useState<{ slug: string; name: string }[]>([])
  const [keys, setKeys] = useState<{ id: string; label: string; tenant_id: string }[]>([])
  const [loading, setLoading] = useState(true)

  const reload = async () => {
    setLoading(true)
    try {
      const [bRes, tRes, kRes] = await Promise.all([listBudgets(), listTenants(), listKeys()])
      setBudgets(bRes?.data ?? [])
      setTenants((tRes ?? []).map((t) => ({ slug: t.slug, name: t.name })))
      setKeys((kRes?.data ?? []).map((k) => ({ id: k.id, label: k.label || k.id, tenant_id: k.tenant_id })))
    } catch {
      toast('Failed to load budgets', 'error')
    } finally {
      setLoading(false)
    }
  }

  useAsyncData(async () => {
    await reload()
    return null
  }, [])

  const [workspace] = useWorkspace()
  const sort = useSort<BudgetDto>('tenant_id', 'asc')
  const rows = useMemo(
    () => sortRows(workspace ? budgets.filter((b) => b.tenant_id === workspace) : budgets, sort.key, sort.dir),
    [budgets, sort, workspace],
  )

  const [modalOpen, setModalOpen] = useState(false)
  const [deleting, setDeleting] = useState<BudgetDto | null>(null)
  const [historyOf, setHistoryOf] = useState<BudgetDto | null>(null)
  const [history, setHistory] = useState<SpendEntryDto[]>([])
  const [busy, setBusy] = useState(false)

  const tenantName = (slug: string) => {
    const t = tenants.find((x) => x.slug === slug)
    return t ? `${t.name} (${slug})` : slug
  }

  const th = (k: keyof BudgetDto, label: string) => (
    <th>
      <SortHeader active={sort.key === k} dir={sort.dir} onClick={() => sort.toggle(k)}>{label}</SortHeader>
    </th>
  )

  const doDelete = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      await deleteBudget(deleting.id)
      toast('Budget deleted', 'success')
      setDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Delete failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  const handleCreate = async (req: CreateBudgetRequest) => {
    try {
      await createBudget(req)
      toast('Budget created', 'success')
      setModalOpen(false)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Create failed', 'error')
    }
  }

  const handleToggle = async (b: BudgetDto) => {
    try {
      await updateBudget(b.id, { enabled: !b.enabled })
      toast(b.enabled ? 'Budget disabled' : 'Budget enabled', 'success')
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Update failed', 'error')
    }
  }

  const showHistory = async (b: BudgetDto) => {
    setHistoryOf(b)
    setHistory([])
    try {
      const res = await budgetHistory(b.id)
      setHistory(res?.data ?? [])
    } catch (e: any) {
      toast(e?.message ?? 'Failed to load history', 'error')
    }
  }

  const spendPercent = (b: BudgetDto): number | null => {
    if (!b.hard_limit || b.hard_limit <= 0) return null
    return Math.min(100, (b.spent ?? 0) / b.hard_limit * 100)
  }

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Budgets</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setModalOpen(true)}>Create Budget</Button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                {th('tenant_id', 'Tenant')}
                {th('scope', 'Scope')}
                {th('type', 'Period')}
                <th>Target</th>
                {th('hard_limit', 'Hard Limit')}
                <th>Spent</th>
                <th>Notify</th>
                {th('enabled', 'Status')}
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((b) => {
                const pct = spendPercent(b)
                return (
                  <tr key={b.id}>
                    <td>{tenantName(b.tenant_id)}</td>
                    <td><Badge value={scopeLabel(b.scope)} /></td>
                    <td>{periodLabel(b)}</td>
                    <td>
                      {b.scope === 'key'
                        ? keys.find((k) => k.id === b.virtual_key_id)?.label ?? b.virtual_key_id
                        : b.scope === 'model' ? (b.model || '—') : 'All models'}
                    </td>
                    <td>{fmtMoney(b.hard_limit, b.currency)}</td>
                    <td>
                      <div className="spend-cell">
                        <div className="progress">
                          <div
                            className={pct !== null && pct >= 90 ? 'progress-fill warn' : 'progress-fill'}
                            style={{ width: `${pct ?? 0}%` }}
                          />
                        </div>
                        <span className="muted">{fmtMoney(b.spent ?? 0, b.currency)}</span>
                      </div>
                    </td>
                    <td>{b.notify_at?.length ? b.notify_at.map((p) => `${p}%`).join(', ') : '—'}</td>
                    <td><Badge value={b.enabled ? 'enabled' : 'disabled'} /></td>
                    <td>
                      <div className="header-actions">
                        <Button size="small" onClick={() => showHistory(b)}>History</Button>
                        <Button size="small" onClick={() => handleToggle(b)}>{b.enabled ? 'Disable' : 'Enable'}</Button>
                        <Button size="small" variant="danger" onClick={() => setDeleting(b)}>Delete</Button>
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!loading && rows.length === 0 && <tr><td colSpan={9}><EmptyState message="No budgets configured" /></td></tr>}
              {loading && <tr><td colSpan={9}><TableSkeleton rows={4} cols={9} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {modalOpen && (
        <CreateBudgetModal
          tenants={tenants}
          keys={keys}
          onClose={() => setModalOpen(false)}
          onCreate={handleCreate}
        />
      )}

      <ConfirmModal
        open={!!deleting}
        title="Delete budget"
        message={`Delete budget for ${deleting?.tenant_id}? Spend enforcement stops immediately.`}
        busy={busy}
        onConfirm={doDelete}
        onCancel={() => setDeleting(null)}
      />

      {historyOf && (
        <div className="modal-backdrop" onClick={() => setHistoryOf(null)}>
          <div className="modal generic-modal" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
            <h3>Spend history — {historyOf.id}</h3>
            <div className="modal-body">
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Time</th>
                      <th>Model</th>
                      <th>Tokens</th>
                      <th>Cost</th>
                    </tr>
                  </thead>
                  <tbody>
                    {history.map((e) => (
                      <tr key={e.id}>
                        <td>{fmtTime(e.created_at)}</td>
                        <td><code>{e.model}</code></td>
                        <td>{e.tokens}</td>
                        <td>{fmtMoney(e.cost, historyOf.currency)}</td>
                      </tr>
                    ))}
                    {history.length === 0 && <tr><td colSpan={4}><EmptyState message="No spend recorded yet" /></td></tr>}
                  </tbody>
                </table>
              </div>
            </div>
            <div className="form-actions">
              <Button variant="primary" onClick={() => setHistoryOf(null)}>Close</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function CreateBudgetModal({
  tenants,
  keys,
  onClose,
  onCreate,
}: {
  tenants: { slug: string; name: string }[]
  keys: { id: string; label: string; tenant_id: string }[]
  onClose: () => void
  onCreate: (req: CreateBudgetRequest) => void
}) {
  const [req, setReq] = useState<CreateBudgetRequest>({
    tenant_id: tenants[0]?.slug ?? '',
    scope: 'tenant',
    type: 'monthly',
    notify_at: [],
  })
  const [err, setErr] = useState('')

  const submit = () => {
    setErr('')
    if (!req.tenant_id) { setErr('Tenant is required'); return }
    if (req.scope === 'key' && !req.virtual_key_id) { setErr('Key scope requires a key'); return }
    if (req.scope === 'model' && !req.model) { setErr('Model scope requires a model'); return }
    onCreate({
      tenant_id: req.tenant_id,
      scope: req.scope,
      type: req.type,
      custom_days: req.type === 'custom' ? (req.custom_days ?? 30) : 0,
      virtual_key_id: req.scope === 'key' ? req.virtual_key_id : undefined,
      model: req.scope === 'model' ? req.model : undefined,
      soft_limit: req.soft_limit,
      hard_limit: req.hard_limit,
      currency: req.currency || undefined,
      notify_at: req.notify_at ?? [],
    })
  }

  const tenantKeys = keys.filter((k) => k.tenant_id === req.tenant_id)

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal generic-modal" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <h3>Create Budget</h3>
        {err && <div className="confirm-dialog" style={{ marginTop: 8 }}><p>{err}</p></div>}
        <div className="modal-body">
          <div className="form-field">
            <label>Tenant</label>
            <select value={req.tenant_id} onChange={(e) => setReq({ ...req, tenant_id: e.target.value, virtual_key_id: undefined })}>
              {tenants.length === 0 && <option value="">No tenants</option>}
              {tenants.map((t) => <option key={t.slug} value={t.slug}>{t.name} ({t.slug})</option>)}
            </select>
          </div>
          <div className="form-field">
            <label>Scope</label>
            <select value={req.scope} onChange={(e) => setReq({ ...req, scope: e.target.value })}>
              <option value="tenant">Tenant</option>
              <option value="key">Virtual key</option>
              <option value="model">Model</option>
            </select>
          </div>
          {req.scope === 'key' && (
            <div className="form-field">
              <label>Key</label>
              <select value={req.virtual_key_id ?? ''} onChange={(e) => setReq({ ...req, virtual_key_id: e.target.value })}>
                <option value="">Select key</option>
                {tenantKeys.map((k) => <option key={k.id} value={k.id}>{k.label}</option>)}
              </select>
            </div>
          )}
          {req.scope === 'model' && (
            <div className="form-field">
              <label>Model</label>
              <input value={req.model ?? ''} onChange={(e) => setReq({ ...req, model: e.target.value })} placeholder="gpt-4o" />
            </div>
          )}
          <div className="form-field">
            <label>Period</label>
            <select value={req.type} onChange={(e) => setReq({ ...req, type: e.target.value })}>
              <option value="monthly">Monthly</option>
              <option value="daily">Daily</option>
              <option value="custom">Custom</option>
            </select>
          </div>
          {req.type === 'custom' && (
            <div className="form-field">
              <label>Days per period</label>
              <input type="number" min={1} value={req.custom_days ?? 30} onChange={(e) => setReq({ ...req, custom_days: parseInt(e.target.value, 10) || 30 })} />
            </div>
          )}
          <div className="form-field">
            <label>Hard limit (USD)</label>
            <input type="number" min={0} step="any" value={req.hard_limit ?? ''} onChange={(e) => setReq({ ...req, hard_limit: e.target.value === '' ? undefined : parseFloat(e.target.value) })} placeholder="100.00" />
          </div>
          <div className="form-field">
            <label>Soft limit (USD)</label>
            <input type="number" min={0} step="any" value={req.soft_limit ?? ''} onChange={(e) => setReq({ ...req, soft_limit: e.target.value === '' ? undefined : parseFloat(e.target.value) })} placeholder="80.00" />
          </div>
          <div className="form-field">
            <label>Notify at % of hard limit (comma separated)</label>
            <input value={(req.notify_at ?? []).join(', ')} onChange={(e) => setReq({ ...req, notify_at: e.target.value.split(',').map((s) => parseFloat(s.trim())).filter((n) => !isNaN(n)) })} placeholder="50, 80, 100" />
          </div>
          <div className="form-field">
            <label>Currency</label>
            <input value={req.currency ?? 'USD'} onChange={(e) => setReq({ ...req, currency: e.target.value })} />
          </div>
        </div>
        <div className="form-actions">
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Create</Button>
        </div>
      </div>
    </div>
  )
}
