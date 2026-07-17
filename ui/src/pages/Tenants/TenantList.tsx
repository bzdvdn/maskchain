import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listTenants, type TenantListItem } from '../../api/tenants'

export function TenantList() {
  const [tenants, setTenants] = useState<TenantListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setLoading(true)
    setError(null)
    listTenants()
      .then(setTenants)
      .catch(() => setError('Failed to load tenants. Please try again.'))
      .finally(() => setLoading(false))
  }, [])

  if (error) {
    return <div className="error-banner">{error}</div>
  }

  if (loading) {
    return <div className="loading">Loading tenants...</div>
  }

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600 }}>All Tenants</h2>
        <Link to="/tenants/new" className="btn btn-primary" style={{ width: 'auto', padding: '8px 16px' }}>
          Create Tenant
        </Link>
      </div>

      {tenants.length === 0 ? (
        <div className="empty-state">
          <p>No tenants yet.</p>
          <Link to="/tenants/new" className="btn btn-primary" style={{ width: 'auto' }}>
            Create your first tenant
          </Link>
        </div>
      ) : (
        <div className="card">
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Slug</th>
                  <th>Name</th>
                  <th>API Keys</th>
                  <th>PII</th>
                  <th>Created</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {tenants.map((t) => (
                  <tr key={t.slug}>
                    <td><code>{t.slug}</code></td>
                    <td>{t.name}</td>
                    <td><code>{t.api_keys[0]?.slice(0, 12)}...</code></td>
                    <td>
                      <span className={`badge ${t.pii_config?.enabled ? 'badge-up' : 'badge-warn'}`}>
                        {t.pii_config?.enabled ? 'On' : 'No rules'}
                      </span>
                    </td>
                    <td>{t.created_at ? new Date(t.created_at).toLocaleDateString() : '—'}</td>
                    <td>
                      <Link to={`/tenants/${t.slug}`} className="btn btn-small">
                        Edit
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
