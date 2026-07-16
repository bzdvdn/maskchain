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
    <div className="tenant-list">
      <div className="page-header">
        <h1>Tenants</h1>
        <Link to="/tenants/new" className="btn btn-primary">
          Create Tenant
        </Link>
      </div>

      {tenants.length === 0 ? (
        <div className="empty-state">
          <p>No tenants yet.</p>
          <Link to="/tenants/new" className="btn btn-primary">
            Create your first tenant
          </Link>
        </div>
      ) : (
        <>
          <table className="profiles-table">
            <thead>
              <tr>
                <th>Slug</th>
                <th>Name</th>
                <th>Auth Header</th>
                <th>PII</th>
                <th>API Keys</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {tenants.map((t) => (
                <tr key={t.slug}>
                  <td><code>{t.slug}</code></td>
                  <td>{t.name}</td>
                  <td><code>{t.auth_header}</code></td>
                  <td>
                    <span className={`status-badge ${t.pii_config?.enabled ? 'status-active' : 'status-disabled'}`}>
                      {t.pii_config?.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td>
                    <code>{t.api_keys[0]?.slice(0, 12)}...</code>
                  </td>
                  <td>
                    <Link to={`/tenants/${t.slug}`} className="btn btn-small">
                      View
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
