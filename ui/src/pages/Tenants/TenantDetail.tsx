import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  getTenant,
  deleteTenant,
  applyCompliancePack,
  getComplianceReport,
  COMPLIANCE_PACKS,
  type TenantResponse,
  type DictionaryItem,
  type ComplianceReport,
} from '../../api/tenants'
import { DictionaryModal } from '../../components/DictionaryModal'
import { ConfirmModal } from '../../components/ConfirmModal'
import { useToast } from '../../components/Toast'
import { Spinner } from '../../components/ui'

export function TenantDetail() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [tenant, setTenant] = useState<TenantResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  const [modalDict, setModalDict] = useState<DictionaryItem | null>(null)
  const [applyingPack, setApplyingPack] = useState<string | null>(null)
  const [reportPack, setReportPack] = useState<string | null>(null)
  const [report, setReport] = useState<ComplianceReport | null>(null)
  const [reportLoading, setReportLoading] = useState(false)

  useEffect(() => {
    if (!slug) return
    setLoading(true)
    getTenant(slug)
      .then(setTenant)
      .catch((err) => {
        if (err.name === 'NotFoundError') setNotFound(true)
      })
      .finally(() => setLoading(false))
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

  function statusBadge(status: string): string {
    switch (status) {
      case 'active':
        return 'badge-up'
      case 'deviated':
        return 'badge-warn'
      default:
        return 'badge-down'
    }
  }

  function formatKeys(keys: string[]): string {
    return keys.map((k) => k.length > 20 ? k.slice(0, 20) + '...' : k).join(', ')
  }

  if (loading) return <Spinner label="Loading tenant..." />

  if (notFound || !tenant) {
    return (
      <div className="not-found">
        <h1>Tenant not found</h1>
        <p>The tenant you are looking for does not exist.</p>
        <Link to="/tenants" className="btn">
          Back to list
        </Link>
      </div>
    )
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600 }}>{tenant.name}</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          <Link to={`/tenants/${tenant.slug}/edit`} className="btn btn-small">
            Edit
          </Link>
          <button
            type="button"
            className="btn btn-small btn-danger"
            onClick={() => setShowConfirm(true)}
          >
            Delete
          </button>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 12 }}>
        <table>
          <tbody>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px', width: 140 }}>Slug</td><td style={{ padding: '8px 12px' }}><code>{tenant.slug}</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Auth Header</td><td style={{ padding: '8px 12px' }}><code>{tenant.auth_header}</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>API Keys</td><td style={{ padding: '8px 12px' }}><code>{formatKeys(tenant.api_keys)}</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Created</td><td style={{ padding: '8px 12px' }}>{new Date(tenant.created_at).toLocaleString()}</td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Updated</td><td style={{ padding: '8px 12px' }}>{new Date(tenant.updated_at).toLocaleString()}</td></tr>
          </tbody>
        </table>
      </div>

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

      {tenant.pii_config && (
        <div className="card">
          <h3>PII Config</h3>
          <table>
            <tbody>
              <tr><td style={{ fontWeight: 600, padding: '8px 12px', width: 140 }}>Enabled</td><td style={{ padding: '8px 12px' }}><span className={`badge ${tenant.pii_config.enabled ? 'badge-up' : 'badge-warn'}`}>{tenant.pii_config.enabled ? 'Yes' : 'No'}</span></td></tr>
              <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Default Action</td><td style={{ padding: '8px 12px' }}>{tenant.pii_config.default_action}</td></tr>
            </tbody>
          </table>
          {tenant.pii_config.rules.length > 0 && (
            <div className="table-wrap" style={{ marginTop: 8 }}>
              <table>
                <thead>
                  <tr><th>Label</th><th>Type</th><th>Pattern</th><th>Action</th></tr>
                </thead>
                <tbody>
                  {tenant.pii_config.rules.map((r, i) => (
                    <tr key={i}>
                      <td><code>{r.label}</code></td>
                      <td>{r.type}</td>
                      <td><code>{r.pattern}</code></td>
                      <td>{r.action}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      <div className="card">
        <h3>Compliance</h3>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 12 }}>
          {COMPLIANCE_PACKS.map((p) => (
            <button
              key={p}
              type="button"
              className="btn btn-small"
              disabled={applyingPack === p}
              onClick={() => handleApplyPack(p)}
            >
              {applyingPack === p ? 'Applying...' : `Apply ${p}`}
            </button>
          ))}
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 8 }}>
          {COMPLIANCE_PACKS.map((p) => (
            <button
              key={p}
              type="button"
              className={`btn btn-small ${reportPack === p ? 'btn-primary' : ''}`}
              onClick={() => handleShowReport(p)}
            >
              {p} report
            </button>
          ))}
        </div>
        {reportLoading && <Spinner label="Loading compliance report..." />}
        {report && (
          <div className="table-wrap" style={{ marginTop: 8 }}>
            <table>
              <thead>
                <tr><th>Detector</th><th>Expected</th><th>Actual</th><th>Masking</th><th>Status</th></tr>
              </thead>
              <tbody>
                {report.rules.map((r, i) => (
                  <tr key={i}>
                    <td><code>{r.detector_type}</code></td>
                    <td>{r.expected_reaction}</td>
                    <td>{r.actual_reaction ?? '—'}</td>
                    <td>{r.masking ? 'yes' : 'no'}</td>
                    <td><span className={`badge ${statusBadge(r.status)}`}>{r.status}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {tenant.dictionaries && tenant.dictionaries.length > 0 && (
        <section>
          <h2 style={{ fontSize: 16, fontWeight: 600, marginBottom: 12 }}>Dictionaries</h2>
          {tenant.dictionaries.map((d, i) => {
            const entries = Array.isArray(d.entries) ? d.entries : []
            return (
              <div key={i} className="card">
                <div className="card-header-row">
                  <h3>{d.name}</h3>
                  <button
                    type="button"
                    className="btn btn-small"
                    onClick={() => setModalDict(d)}
                  >
                    View all ({entries.length})
                  </button>
                </div>
                <p style={{ fontSize: 13, color: 'var(--text-muted)', marginBottom: 4 }}>Match mode: <code>{d.match_mode}</code></p>
                <ul>
                  {entries.slice(0, 10).map((e: any, j: number) => (
                    <li key={j} style={{ padding: '4px 0', borderBottom: '1px solid var(--border)', fontSize: 13, listStyle: 'none' }}>
                      {typeof e === 'string' ? e : JSON.stringify(e)}
                    </li>
                  ))}
                  {entries.length > 10 && (
                    <li style={{ padding: '4px 0', fontSize: 13, listStyle: 'none', color: 'var(--text-muted)' }}>...and {entries.length - 10} more</li>
                  )}
                </ul>
              </div>
            )
          })}
        </section>
      )}

      {modalDict && (
        <DictionaryModal dict={modalDict} onClose={() => setModalDict(null)} />
      )}
    </div>
  )
}
