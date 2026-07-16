import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { getTenant, deleteTenant, type TenantResponse, type DictionaryItem } from '../../api/tenants'
import { DictionaryModal } from '../../components/DictionaryModal'

export function TenantDetail() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const [tenant, setTenant] = useState<TenantResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  const [modalDict, setModalDict] = useState<DictionaryItem | null>(null)

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
      navigate('/tenants')
    } catch {
      setDeleting(false)
      setShowConfirm(false)
    }
  }

  function formatKeys(keys: string[]): string {
    return keys.map((k) => k.length > 20 ? k.slice(0, 20) + '...' : k).join(', ')
  }

  if (loading) return <div className="loading">Loading tenant...</div>

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
    <div className="profile-detail">
      <div className="page-header">
        <h1>{tenant.name}</h1>
        <div className="header-actions">
          <Link to={`/tenants/${tenant.slug}/edit`} className="btn">
            Edit
          </Link>
          <button
            type="button"
            className="btn btn-danger"
            onClick={() => setShowConfirm(true)}
          >
            Delete
          </button>
        </div>
      </div>

      <div className="detail-grid">
        <div className="detail-field">
          <label>Slug</label>
          <span><code>{tenant.slug}</code></span>
        </div>
        <div className="detail-field">
          <label>Auth Header</label>
          <span><code>{tenant.auth_header}</code></span>
        </div>
        <div className="detail-field">
          <label>API Keys</label>
          <span><code>{formatKeys(tenant.api_keys)}</code></span>
        </div>
        <div className="detail-field">
          <label>Created</label>
          <span>{new Date(tenant.created_at).toLocaleString()}</span>
        </div>
        <div className="detail-field">
          <label>Updated</label>
          <span>{new Date(tenant.updated_at).toLocaleString()}</span>
        </div>
      </div>

      {showConfirm && (
        <div className="confirm-dialog">
          <p>
            Are you sure you want to delete tenant "{tenant.name}"? This
            action cannot be undone.
          </p>
          <div className="confirm-actions">
            <button
              type="button"
              className="btn"
              onClick={() => setShowConfirm(false)}
              disabled={deleting}
            >
              Cancel
            </button>
            <button
              type="button"
              className="btn btn-danger"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? 'Deleting...' : 'Confirm Delete'}
            </button>
          </div>
        </div>
      )}

      {tenant.pii_config && (
        <section>
          <h2>PII Config</h2>
          <div className="card">
            <p>
              <strong>Enabled:</strong>{' '}
              <span className={`status-badge ${tenant.pii_config.enabled ? 'status-active' : 'status-disabled'}`}>
                {tenant.pii_config.enabled ? 'Yes' : 'No'}
              </span>
            </p>
            <p><strong>Default Action:</strong> {tenant.pii_config.default_action}</p>
            {tenant.pii_config.rules.length > 0 && (
              <>
                <p><strong>Rules ({tenant.pii_config.rules.length})</strong></p>
                <ul>
                  {tenant.pii_config.rules.map((r, i) => (
                    <li key={i}>
                      <code>{r.label}</code> — type: {r.type}, pattern: {r.pattern}, action: {r.action}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
        </section>
      )}

      {tenant.dictionaries && tenant.dictionaries.length > 0 && (
        <section>
          <h2>Dictionaries</h2>
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
                <p>Match mode: <code>{d.match_mode}</code></p>
                <p>Entries: {entries.length}</p>
                <ul>
                  {entries.slice(0, 10).map((e: any, j: number) => (
                    <li key={j}>
                      {typeof e === 'string' ? e : JSON.stringify(e)}
                    </li>
                  ))}
                  {entries.length > 10 && (
                    <li className="text-muted">...and {entries.length - 10} more</li>
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
