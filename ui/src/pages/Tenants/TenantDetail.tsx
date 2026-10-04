import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import {
  getTenant,
  deleteTenant,
  applyCompliancePack,
  getComplianceReport,
  type TenantResponse,
  type DictionaryItem,
  type ComplianceReport,
} from '../../api/tenants'
import { useShieldCatalog } from '../../hooks/useShieldCatalog'
import { listConversations } from '../../api/conversations'
import { DictionaryModal } from '../../components/DictionaryModal'
import { ConfirmModal } from '../../components/ConfirmModal'
import { useToast } from '../../components/Toast'
import { Button, Spinner, StatusPill, type StatusTone } from '../../components/ui'
import { relativeTime, absDate } from '../../utils/format'

type Tab = 'overview' | 'policies' | 'compliance' | 'activity'

function ruleTone(status: string): StatusTone {
  if (status === 'active') return 'green'
  if (status === 'deviated') return 'amber'
  return 'red'
}

export function TenantDetail() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [tenant, setTenant] = useState<TenantResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [tab, setTab] = useState<Tab>('overview')
  const [deleting, setDeleting] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  const [modalDict, setModalDict] = useState<DictionaryItem | null>(null)
  const [applyingPack, setApplyingPack] = useState<string | null>(null)
  const [reportPack, setReportPack] = useState<string | null>(null)
  const [report, setReport] = useState<ComplianceReport | null>(null)
  const [reportLoading, setReportLoading] = useState(false)
  const [activity, setActivity] = useState<{ id: string; model: string; status: string; created_at: string }[]>([])
  const { packs } = useShieldCatalog()

  useEffect(() => {
    if (!slug) return
    setLoading(true)
    getTenant(slug)
      .then(setTenant)
      .catch((err) => {
        if (err.name === 'NotFoundError') setNotFound(true)
      })
      .finally(() => setLoading(false))
    listConversations(1, 30, { tenant_id: slug })
      .then((res) => setActivity(Array.isArray(res.items) ? res.items.map((c) => ({ id: c.id, model: c.model, status: c.status, created_at: c.created_at })) : []))
      .catch(() => {})
  }, [slug])

  async function handleDelete() {
    if (!slug) return
    setDeleting(true)
    try {
      await deleteTenant(slug)
      toast(`Tenant "${slug}" deleted`, 'success')
      navigate('/tenants')
    } catch {
      toast('Failed to delete tenant', 'error')
      setDeleting(false)
      setShowConfirm(false)
    }
  }

  async function handleApplyPack(packKey: string) {
    if (!slug) return
    setApplyingPack(packKey)
    try {
      await applyCompliancePack(slug, packKey)
      toast(`Compliance pack "${packKey}" applied`, 'success')
      setReportPack(packKey)
      setTenant(await getTenant(slug))
      setReport(await getComplianceReport(slug, packKey))
    } catch {
      toast('Failed to apply compliance pack', 'error')
    } finally {
      setApplyingPack(null)
    }
  }

  async function handleShowReport(packKey: string) {
    if (!slug) return
    setReportPack(packKey)
    setReportLoading(true)
    setReport(null)
    try {
      setReport(await getComplianceReport(slug, packKey))
    } catch {
      toast('Failed to load compliance report', 'error')
    } finally {
      setReportLoading(false)
    }
  }

  const deviated = report ? report.rules.filter((r) => r.status !== 'active') : []
  const hasDeviation = deviated.length > 0

  if (loading) return <Spinner label="Loading tenant..." />

  if (notFound || !tenant) {
    return (
      <div className="not-found">
        <h1>Tenant not found</h1>
        <p>The tenant you are looking for does not exist.</p>
        <Link to="/tenants" className="btn">Back to list</Link>
      </div>
    )
  }

  return (
    <div>
      <div className="u-between u-mb18">
        <div className="u-flex-lg">
          <h2 className="page-name">{tenant.name}</h2>
          <StatusPill tone="green">active</StatusPill>
          <span className="muted mono slug-tag">{tenant.slug}</span>
        </div>
        <div className="u-flex">
          <Link to={`/tenants/${tenant.slug}/edit`} className="btn btn-small">Edit</Link>
          <button type="button" className="btn btn-small btn-danger" onClick={() => setShowConfirm(true)}>Delete</button>
        </div>
      </div>

      {hasDeviation && reportPack && (
        <div className="banner amber">
          <AlertTriangle size={16} />
          <div className="text"><b>Compliance deviation.</b> Pack <b>{reportPack}</b> has {deviated.length} rule{deviated.length > 1 ? 's' : ''} out of sync — review the diff and re-apply.</div>
          <Button size="small" onClick={() => setTab('compliance')}>Review diff</Button>
          <Button size="small" variant="primary" onClick={() => handleApplyPack(reportPack)} disabled={applyingPack === reportPack}>
            <RefreshCw size={13} /> Re-apply
          </Button>
        </div>
      )}

      <div className="tabs" role="tablist">
        {([['overview', 'Overview'], ['policies', 'Policies & Shield'], ['compliance', 'Compliance'], ['activity', 'Activity']] as [Tab, string][]).map(([key, label]) => (
          <button key={key} role="tab" aria-selected={tab === key} className={`tab${tab === key ? ' active' : ''}`} onClick={() => setTab(key)}>
            {label}
            {key === 'compliance' && hasDeviation && <span className="pill amber mini">{deviated.length}</span>}
          </button>
        ))}
      </div>

      {tab === 'overview' && (
        <div className="card">
          <div className="kv">
            <div className="k">Slug</div><div className="v mono">{tenant.slug}</div>
            <div className="k">Auth header</div><div className="v mono">{tenant.auth_header}</div>
            <div className="k">Retention mode</div><div className="v">{tenant.retention_mode ?? 'full'}</div>
            <div className="k">Created</div><div className="v">{absDate(tenant.created_at)}</div>
            <div className="k">Updated</div><div className="v">{absDate(tenant.updated_at)}</div>
          </div>
        </div>
      )}

      {tab === 'policies' && (
        <div className="stack">
          {tenant.pii_config && (
            <div className="card">
              <div className="card-header-row"><h3>PII shield</h3><StatusPill tone={tenant.pii_config.enabled ? 'green' : 'gray'}>{tenant.pii_config.enabled ? 'enabled' : 'disabled'}</StatusPill></div>
              <div className="kv u-mb12">
                <div className="k">Default action</div><div className="v mono">{tenant.pii_config.default_action}</div>
              </div>
              {Array.isArray(tenant.pii_config.rules) && tenant.pii_config.rules.length > 0 && (
                <div className="table-wrap">
                  <table className="tbl">
                    <thead><tr><th>Label</th><th>Type</th><th>Pattern</th><th>Action</th></tr></thead>
                    <tbody>
                      {tenant.pii_config.rules.map((r, i) => (
                        <tr key={i}><td className="mono">{r.label}</td><td>{r.type}</td><td className="mono">{r.pattern}</td><td>{r.action}</td></tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {tenant.dictionaries && tenant.dictionaries.length > 0 && (
            <div className="card">
              <div className="card-header-row"><h3>Dictionaries</h3></div>
              <div className="stack u-gap10">
                {tenant.dictionaries.map((d, i) => {
                  const entries = Array.isArray(d.entries) ? d.entries : []
                  return (
                    <div key={i} className="box">
                      <div className="card-header-row">
                        <h3 className="u-h3">{d.name} <span className="muted">· {d.match_mode}</span></h3>
                        <Button size="small" onClick={() => setModalDict(d)}>View all ({entries.length})</Button>
                      </div>
                      <div className="muted dict-scroll">
                        {entries.slice(0, 8).map((e: any, j: number) => <div key={j}>{typeof e === 'string' ? e : JSON.stringify(e)}</div>)}
                        {entries.length > 8 && <div>…and {entries.length - 8} more</div>}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {tab === 'compliance' && (
        <div className="stack">
          <div className="card">
            <div className="card-header-row"><h3>Apply compliance packs</h3></div>
            <div className="u-wrap">
              {packs.length === 0 && <span className="muted">No compliance packs available.</span>}
              {packs.map((p) => (
                <Button key={p.key} size="small" onClick={() => handleApplyPack(p.key)} disabled={applyingPack === p.key}>
                  {applyingPack === p.key ? 'Applying…' : `Apply ${p.key}`}
                </Button>
              ))}
            </div>
          </div>
          <div className="card">
            <div className="card-header-row"><h3>Reports</h3></div>
            <div className="u-wrap u-mb12">
              {packs.map((p) => (
                <Button
                  key={p.key}
                  size="small"
                  variant={reportPack === p.key ? 'primary' : 'default'}
                  onClick={() => handleShowReport(p.key)}
                >
                  {p.key} report
                </Button>
              ))}
            </div>
            {reportLoading && <Spinner label="Loading compliance report..." />}
            {report && (
              <div className="table-wrap">
                <table className="tbl">
                  <thead><tr><th>Detector</th><th>Expected</th><th>Actual</th><th>Masking</th><th>Status</th></tr></thead>
                  <tbody>
                    {report.rules.map((r, i) => (
                      <tr key={i}>
                        <td className="mono">{r.detector_type}</td>
                        <td>{r.expected_reaction}</td>
                        <td>{r.actual_reaction ?? '—'}</td>
                        <td>{r.masking ? 'yes' : 'no'}</td>
                        <td><StatusPill tone={ruleTone(r.status)}>{r.status}</StatusPill></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}

      {tab === 'activity' && (
        <div className="card">
          <div className="card-header-row"><h3>Recent requests</h3><span className="muted">last 30</span></div>
          {activity.length === 0 ? (
            <div className="empty-state u-center">No traffic for this tenant yet.</div>
          ) : (
            <div className="table-wrap">
              <table className="tbl">
                <thead><tr><th>Model</th><th>Status</th><th>When</th></tr></thead>
                <tbody>
                  {activity.map((a) => (
                    <tr key={a.id}>
                      <td className="mono">{a.model || '—'}</td>
                      <td><StatusPill tone={a.status === 'ok' ? 'green' : a.status === 'blocked' ? 'amber' : 'red'}>{a.status}</StatusPill></td>
                      <td className="muted">{relativeTime(a.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {showConfirm && (
        <ConfirmModal
          open={showConfirm}
          title="Delete tenant"
          message={`Are you sure you want to delete tenant "${tenant.name}"? This action cannot be undone.`}
          confirmLabel="Delete"
          busy={deleting}
          onConfirm={handleDelete}
          onCancel={() => setShowConfirm(false)}
        />
      )}

      {modalDict && <DictionaryModal dict={modalDict} onClose={() => setModalDict(null)} />}
    </div>
  )
}