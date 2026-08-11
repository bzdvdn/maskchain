import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Badge, Button, EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { CopyButton } from '../components/CopyButton'
import { useToast } from '../components/Toast'
import { useSort, sortRows } from '../hooks/useSort'
import { createKey, deleteKey, listKeys, updateKey, type CreateKeyRequest, type VirtualKeyDto } from '../api/keys'
import { listTenants } from '../api/tenants'

function fmtTime(iso: string | undefined | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString()
}

function modelList(models: string[] | undefined): string {
  if (!models || models.length === 0) return '—'
  return models.join(', ')
}

export function Keys() {
  const { toast } = useToast()
  const [keys, setKeys] = useState<VirtualKeyDto[]>([])
  const [tenants, setTenants] = useState<{ slug: string; name: string }[]>([])
  const [loading, setLoading] = useState(true)

  const reload = async () => {
    setLoading(true)
    try {
      const [kRes, tRes] = await Promise.all([listKeys(), listTenants()])
      setKeys(kRes?.data ?? [])
      setTenants((tRes ?? []).map((t) => ({ slug: t.slug, name: t.name })))
    } catch {
      toast('Failed to load keys', 'error')
    } finally {
      setLoading(false)
    }
  }

  useAsyncData(async () => {
    await reload()
    return null
  }, [])

  const sort = useSort<VirtualKeyDto>('tenant_id', 'asc')
  const sortState = sort
  const rows = useMemo(() => sortRows(keys, sortState.key, sortState.dir), [keys, sortState])

  const [modalOpen, setModalOpen] = useState(false)
  const [createdKey, setCreatedKey] = useState<{ key: string; label: string } | null>(null)
  const [deleting, setDeleting] = useState<VirtualKeyDto | null>(null)
  const [busy, setBusy] = useState(false)

  const tenantName = (slug: string) => {
    const t = tenants.find((x) => x.slug === slug)
    return t ? `${t.name} (${slug})` : slug
  }

  const th = (k: keyof VirtualKeyDto, label: string) => (
    <th>
      <SortHeader active={sort.key === k} dir={sort.dir} onClick={() => sort.toggle(k)}>{label}</SortHeader>
    </th>
  )

  const doDelete = async () => {
    if (!deleting) return
    setBusy(true)
    try {
      await deleteKey(deleting.id)
      toast(`Key "${deleting.label || deleting.id}" revoked`, 'success')
      setDeleting(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Revoke failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  const handleCreate = async (req: CreateKeyRequest) => {
    try {
      const resp = await createKey(req)
      setCreatedKey({ key: resp.key, label: resp.label || resp.id })
      setModalOpen(false)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Create failed', 'error')
    }
  }

  const handleToggle = async (k: VirtualKeyDto) => {
    try {
      await updateKey(k.id, { enabled: !k.enabled })
      toast(k.enabled ? 'Key disabled' : 'Key enabled', 'success')
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Update failed', 'error')
    }
  }

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Virtual Keys</h3>
          <div className="header-actions">
            <Button size="small" onClick={() => setModalOpen(true)}>Create Key</Button>
          </div>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                {th('tenant_id', 'Tenant')}
                {th('label', 'Label')}
                <th>Allowed Models</th>
                <th>Blocked Models</th>
                {th('enabled', 'Status')}
                <th>Expires</th>
                {th('created_at', 'Created')}
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((k) => (
                <tr key={k.id}>
                  <td>{tenantName(k.tenant_id)}</td>
                  <td>{k.label || k.id}</td>
                  <td><code>{modelList(k.allowed_models)}</code></td>
                  <td><code>{modelList(k.blocked_models)}</code></td>
                  <td><Badge value={k.enabled ? 'enabled' : 'disabled'} /></td>
                  <td>{fmtTime(k.expires_at)}</td>
                  <td>{fmtTime(k.created_at)}</td>
                  <td>
                    <div className="header-actions">
                      <Button size="small" onClick={() => handleToggle(k)}>{k.enabled ? 'Disable' : 'Enable'}</Button>
                      <Button size="small" variant="danger" onClick={() => setDeleting(k)}>Revoke</Button>
                    </div>
                  </td>
                </tr>
              ))}
              {!loading && rows.length === 0 && <tr><td colSpan={8}><EmptyState message="No virtual keys" /></td></tr>}
              {loading && <tr><td colSpan={8}><TableSkeleton rows={4} cols={8} /></td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {modalOpen && (
        <CreateKeyModal
          tenants={tenants}
          onClose={() => setModalOpen(false)}
          onCreate={handleCreate}
        />
      )}

      {createdKey && (
        <div className="modal-backdrop">
          <div className="modal generic-modal" role="dialog" aria-modal="true">
            <h3>Key created</h3>
            <div className="modal-body">
              <p>Copy the key now — it will not be shown again.</p>
              <div className="form-field">
                <label>Key ({createdKey.label})</label>
                <div className="copy-row">
                  <code>{createdKey.key}</code>
                  <CopyButton text={createdKey.key} label="Copy" />
                </div>
              </div>
            </div>
            <div className="form-actions">
              <Button variant="primary" onClick={() => setCreatedKey(null)}>Done</Button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        open={!!deleting}
        title="Revoke key"
        message={`Revoke "${deleting?.label || deleting?.id}"? The key will stop working immediately.`}
        busy={busy}
        onConfirm={doDelete}
        onCancel={() => setDeleting(null)}
      />
    </div>
  )
}

function CreateKeyModal({
  tenants,
  onClose,
  onCreate,
}: {
  tenants: { slug: string; name: string }[]
  onClose: () => void
  onCreate: (req: CreateKeyRequest) => void
}) {
  const [req, setReq] = useState<CreateKeyRequest>({
    tenant_id: tenants[0]?.slug ?? '',
    label: '',
    allowed_models: [],
    blocked_models: [],
  })
  const [err, setErr] = useState('')

  const submit = () => {
    setErr('')
    if (!req.tenant_id) { setErr('Tenant is required'); return }
    onCreate({
      tenant_id: req.tenant_id,
      label: req.label,
      allowed_models: req.allowed_models ?? [],
      blocked_models: req.blocked_models ?? [],
    })
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal generic-modal" role="dialog" aria-modal="true" onClick={(e) => e.stopPropagation()}>
        <h3>Create Key</h3>
        {err && <div className="confirm-dialog" style={{ marginTop: 8 }}><p>{err}</p></div>}
        <div className="modal-body">
          <div className="form-field">
            <label>Tenant</label>
            <select value={req.tenant_id} onChange={(e) => setReq({ ...req, tenant_id: e.target.value })}>
              {tenants.length === 0 && <option value="">No tenants</option>}
              {tenants.map((t) => <option key={t.slug} value={t.slug}>{t.name} ({t.slug})</option>)}
            </select>
          </div>
          <div className="form-field">
            <label>Label</label>
            <input value={req.label ?? ''} onChange={(e) => setReq({ ...req, label: e.target.value })} placeholder="production-integration" />
          </div>
          <div className="form-field">
            <label>Allowed Models (comma separated, empty = all)</label>
            <input value={(req.allowed_models ?? []).join(', ')} onChange={(e) => setReq({ ...req, allowed_models: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} placeholder="gpt-4o, claude-3-5-sonnet" />
          </div>
          <div className="form-field">
            <label>Blocked Models (comma separated)</label>
            <input value={(req.blocked_models ?? []).join(', ')} onChange={(e) => setReq({ ...req, blocked_models: e.target.value.split(',').map((s) => s.trim()).filter(Boolean) })} />
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