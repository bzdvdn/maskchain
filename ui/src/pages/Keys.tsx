import { useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Plus, Search, ClipboardCopy, KeyRound } from 'lucide-react'
import { AsyncSection, Button, ChipInput, ChipTag, Modal, Pagination, ProgressBar, Segmented, StatusPill, Switch, TableSkeleton } from '../components/ui'
import { ConfirmModal } from '../components/ConfirmModal'
import { CopyButton } from '../components/CopyButton'
import { useToast } from '../components/Toast'
import { relativeTime, money } from '../utils/format'
import { createKey, deleteKey, listKeysPage, rotateKey, updateKey, type CreateKeyRequest, type VirtualKeyDto } from '../api/keys'
import { listTenants } from '../api/tenants'
import { useWorkspace } from '../hooks/useWorkspace'
import { useUrlFilters } from '../hooks/useUrlFilters'
import { useGatewayBase } from '../hooks/useGatewayBase'

const EXPIRY_PRESETS: { key: string; label: string; value: () => string | null }[] = [
  { key: '7d', label: '7 days', value: () => new Date(Date.now() + 7 * 86400_000).toISOString() },
  { key: '30d', label: '30 days', value: () => new Date(Date.now() + 30 * 86400_000).toISOString() },
  { key: 'never', label: 'Never', value: () => null },
]

function tenantName(slug: string, tenants: { slug: string; name: string }[]): string {
  const t = tenants.find((x) => x.slug === slug)
  return t ? `${t.name} (${slug})` : slug
}

const PER_PAGE = 20

export function Keys() {
  const { toast } = useToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const gatewayBase = useGatewayBase()
  const endpointBase = gatewayBase ? `${gatewayBase}/api/v1` : ''

  const [keys, setKeys] = useState<VirtualKeyDto[]>([])
  const [tenants, setTenants] = useState<{ slug: string; name: string }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<unknown>(null)

  const url = useUrlFilters()
  const [search, setSearch] = useState(() => url.get('search'))
  const [page, setPage] = useState(() => url.getInt('page', 1))
  const [total, setTotal] = useState(0)
  const [workspace] = useWorkspace()
  const [tenantFilter, setTenantFilter] = useState(workspace)
  const [statusFilter, setStatusFilter] = useState('')

  const [modalOpen, setModalOpen] = useState(false)
  const drawerOpenedRef = useRef(false)
  const [createdKey, setCreatedKey] = useState<{ key: string; label: string } | null>(null)
  const [deleting, setDeleting] = useState<VirtualKeyDto | null>(null)
  const [rotating, setRotating] = useState<VirtualKeyDto | null>(null)
  const [busy, setBusy] = useState(false)

  const reload = async () => {
    setLoading(true)
    setError(null)
    try {
      const [kRes, tRes] = await Promise.all([listKeysPage({ page, perPage: PER_PAGE, search }), listTenants()])
      setKeys(kRes.items)
      setTotal(kRes.total)
      setTenants(Array.isArray(tRes) ? tRes.map((t) => ({ slug: t.slug, name: t.name })) : [])
    } catch (err) {
      setError(err)
      toast('Failed to load keys', 'error')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    setTenantFilter(workspace)
  }, [workspace])

  useEffect(() => {
    if (searchParams.get('create') === '1' && !drawerOpenedRef.current) {
      drawerOpenedRef.current = true
      setModalOpen(true)
      setSearchParams({}, { replace: true })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    reload()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page, search])

  const rows = useMemo(() => {
    return keys
      .filter((k) => tenantFilter === '' || k.tenant_id === tenantFilter)
      .filter((k) => statusFilter === '' || (statusFilter === 'enabled' ? k.enabled : !k.enabled))
      .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
  }, [keys, tenantFilter, statusFilter])

  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))

  function onSearch(value: string) {
    setSearch(value)
    setPage(1)
    url.set({ search: value || undefined, page: undefined })
  }

  function changePage(p: number) {
    setPage(p)
    url.set({ page: p > 1 ? p : undefined })
  }

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

  const doRotate = async () => {
    if (!rotating) return
    setBusy(true)
    try {
      const resp = await rotateKey(rotating.id)
      setCreatedKey({ key: resp.key, label: resp.label || resp.id })
      setRotating(null)
      await reload()
    } catch (e: any) {
      toast(e?.message ?? 'Rotate failed', 'error')
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
      setKeys((prev) => prev.map((x) => (x.id === k.id ? { ...x, enabled: !k.enabled } : x)))
    } catch (e: any) {
      toast(e?.message ?? 'Update failed', 'error')
    }
  }

  const modelChips = (models: string[] | undefined) => {
    if (!models || models.length === 0) return <span className="muted">all</span>
    const shown = models.slice(0, 3)
    const rest = models.length - shown.length
    return (
      <>
        {shown.map((m) => <ChipTag key={m}>{m}</ChipTag>)}
        {rest > 0 && <ChipTag>{`+${rest}`}</ChipTag>}
      </>
    )
  }

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Connect your app</h3>
          <span className="muted u-flex">
            <span className="live-dot" /> REST · SSE streaming
          </span>
        </div>
        <div className="endpoint">
          <div className="box">
            <div className="u-label">Base URL</div>
            <div className="copy-row">
              <code>{endpointBase || '…'}</code>
              <CopyButton text={endpointBase} label="Copy" />
            </div>
            <div className="divider" />
            <div className="copy-row">
              <span className="muted">curl</span>
              <code className="u-ellipsis">
                {`curl ${endpointBase || '…'}/chat/completions -H "Authorization: Bearer sk-mc-…"`}
              </code>
              <CopyButton
                text={`curl ${endpointBase || ''}/chat/completions \\\n  -H "Authorization: Bearer sk-mc-…"`}
                label="Copy"
              />
            </div>
          </div>
          <div className="box">
            <div className="label">Keys are hashed at rest</div>
            <p className="muted-sm u-mt4">
              Provider secrets are encrypted with AES-256-GCM; virtual keys are stored as SHA-256
              hashes and shown exactly once at creation.
            </p>
          </div>
        </div>
      </div>

      {!loading && keys.length === 0 && (
        <div className="onboard u-mb16">
          <div className="u-flex">
            <KeyRound size={18} className="ic-accent" />
            <div>
              <div className="onboard-title">Get your first key</div>
              <p className="muted-sm">Issue a tenant-scoped key and hand your app the curl snippet above — traffic flows through shield, routing and budgets automatically.</p>
            </div>
          </div>
          <div className="u-flex">
            <Button variant="primary" onClick={() => setModalOpen(true)}><Plus size={14} /> Create virtual key</Button>
          </div>
        </div>
      )}

      <div className="toolbar">
        <div className="search">
          <Search size={14} className="search-icon" />
          <input placeholder="Search by label, tenant or id…" value={search} onChange={(e) => onSearch(e.target.value)} aria-label="Search keys" />
        </div>
        <select className="select" value={tenantFilter} onChange={(e) => setTenantFilter(e.target.value)} aria-label="Tenant filter">
          <option value="">All tenants</option>
          {tenants.map((t) => <option key={t.slug} value={t.slug}>{t.name}</option>)}
        </select>
        <select className="select" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} aria-label="Status filter">
          <option value="">All statuses</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
        </select>
        <div className="u-grow" />
        <Button size="small" onClick={() => setModalOpen(true)}><Plus size={13} /> Create key</Button>
      </div>

      <div className="card table-card">
        <div className="table-wrap table-flush">
          <table className="tbl">
            <thead>
              <tr>
                <th>Tenant</th>
                <th>Label</th>
                <th>Models</th>
                <th className="budget-col">Budget &amp; Spend</th>
                <th>Status</th>
                <th>Expires</th>
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
              emptyMessage="No virtual keys match the current filters."
              skeleton={<TableSkeleton rows={4} cols={7} />}
            >
              {rows.map((k) => {
                const pct = k.budget_cap ? Math.round((k.spent / k.budget_cap) * 100) : null
                return (
                  <tr key={k.id}>
                    <td><span className="u-fw">{tenantName(k.tenant_id, tenants)}</span></td>
                    <td><code>{k.label || k.id.slice(0, 8)}</code></td>
                    <td>{modelChips(k.allowed_models)}</td>
                    <td>
                      {pct === null ? (
                        <span className="muted">—</span>
                      ) : (
                        <div className="prog-cell">
                          <div className="meta">
                            <span className="mono">{money(k.spent)}</span>
                            <span className={`mono ${pct >= 90 ? '' : 'muted'}`}>{pct}%</span>
                          </div>
                          <ProgressBar percentage={pct} ariaLabel={`${k.label || k.id} spend`} />
                          <div className="meta"><span>of {money(k.budget_cap ?? 0)}</span></div>
                        </div>
                      )}
                    </td>
                    <td>
                      <span className="u-flex">
                        <StatusPill tone={k.enabled ? 'green' : 'gray'}>{k.enabled ? 'enabled' : 'disabled'}</StatusPill>
                        <Switch checked={k.enabled} onChange={() => handleToggle(k)} label={`Toggle ${k.label || k.id}`} />
                      </span>
                    </td>
                    <td className="muted">{k.expires_at ? relativeTime(k.expires_at) : 'Never'}</td>
                    <td>
                      <div className="u-actions">
                        <button className="icon-btn sm" title="Copy key id" onClick={() => { navigator.clipboard.writeText(k.id); toast('Key id copied', 'success') }}>
                          <ClipboardCopy size={13} />
                        </button>
                        <Button size="small" onClick={() => setRotating(k)}>Rotate</Button>
                        <Button size="small" variant="danger" onClick={() => setDeleting(k)}>Revoke</Button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </AsyncSection>
          </table>
        </div>
      </div>

      {total > PER_PAGE && (
        <Pagination page={page} totalPages={totalPages} total={total} onPage={changePage} />
      )}

      {modalOpen && (
        <CreateKeyModal
          tenants={tenants}
          onClose={() => setModalOpen(false)}
          onCreate={handleCreate}
        />
      )}

      {createdKey && (
        <Modal
          open
          onClose={() => setCreatedKey(null)}
          closeOnBackdrop={false}
          title="Key created"
          footer={<Button variant="primary" onClick={() => setCreatedKey(null)}>Done</Button>}
        >
          <p className="muted">Copy the key now — it will not be shown again.</p>
          <div className="form-field">
            <label>Key ({createdKey.label})</label>
            <div className="copy-row">
              <code>{createdKey.key}</code>
              <CopyButton text={createdKey.key} label="Copy" />
            </div>
          </div>
        </Modal>
      )}

      <ConfirmModal
        open={!!rotating}
        title="Rotate key"
        message={`Rotate "${rotating?.label || rotating?.id}"? A new secret is issued and the old one stops working immediately.`}
        confirmLabel="Rotate"
        busy={busy}
        onConfirm={doRotate}
        onCancel={() => setRotating(null)}
      />

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

interface CreateModalProps {
  tenants: { slug: string; name: string }[]
  onClose: () => void
  onCreate: (req: CreateKeyRequest) => void
}

function CreateKeyModal({ tenants, onClose, onCreate }: CreateModalProps) {
  const [tenant, setTenant] = useState(tenants[0]?.slug ?? '')
  const [label, setLabel] = useState('')
  const [allowed, setAllowed] = useState<string[]>([])
  const [blocked, setBlocked] = useState<string[]>([])
  const [budget, setBudget] = useState('')
  const [expiry, setExpiry] = useState('30d')
  const [err, setErr] = useState('')

  useEffect(() => {
    if (!tenant && tenants.length > 0) {
      setTenant(tenants[0].slug)
    }
  }, [tenants, tenant])

  const submit = () => {
    setErr('')
    if (!tenant) { setErr('Tenant is required'); return }
    const budgetNum = budget ? Number(budget) : undefined
    if (budget && (!Number.isFinite(budgetNum) || (budgetNum as number) <= 0)) { setErr('Budget must be a positive number'); return }
    const preset = EXPIRY_PRESETS.find((p) => p.key === expiry)
    onCreate({
      tenant_id: tenant,
      label: label || undefined,
      allowed_models: allowed.length > 0 ? allowed : undefined,
      blocked_models: blocked.length > 0 ? blocked : undefined,
      budget_cap: budgetNum,
      expires_at: preset?.value() ?? null,
    })
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Create key"
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button variant="primary" onClick={submit}>Create</Button>
        </>
      }
    >
      {err && <div className="confirm-dialog u-mt8"><p>{err}</p></div>}
          <div className="form-field">
            <label>Tenant</label>
            <select value={tenant} onChange={(e) => setTenant(e.target.value)}>
              {tenants.length === 0 && <option value="">No tenants</option>}
              {tenants.map((t) => <option key={t.slug} value={t.slug}>{t.name} ({t.slug})</option>)}
            </select>
          </div>
          <div className="form-field">
            <label>Label</label>
            <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="e.g. payments-gw, ci-pipeline" />
          </div>
          <div className="form-field">
            <label>Allowed models (empty = all)</label>
            <ChipInput value={allowed} onChange={setAllowed} placeholder="press Enter to add a model" />
          </div>
          <div className="form-field">
            <label>Blocked models</label>
            <ChipInput value={blocked} onChange={setBlocked} placeholder="press Enter to add a model" />
          </div>
          <div className="form-field">
            <label>Budget limit (USD)</label>
            <input inputMode="numeric" value={budget} onChange={(e) => setBudget(e.target.value)} placeholder="e.g. 500" />
          </div>
          <div className="form-field">
            <label>Expiry</label>
            <Segmented
              options={EXPIRY_PRESETS.map((p) => ({ key: p.key, label: p.label }))}
              value={expiry}
              onChange={setExpiry}
              ariaLabel="Expiry"
            />
          </div>
    </Modal>
  )
}