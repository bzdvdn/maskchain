import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listTenants, type TenantListItem } from '../../api/tenants'
import { Badge, EmptyState, Spinner } from '../../components/ui'
import { useAsyncData } from '../../hooks/useAsyncData'

export function TenantList() {
  const [error, setError] = useState<string | null>(null)
  const { data: tenants, loading } = useAsyncData<TenantListItem[]>(
    listTenants,
    [],
    { onError: () => setError('Failed to load tenants. Please try again.') },
  )

  useEffect(() => {
    if (!error) return
    const t = setTimeout(() => setError(null), 4000)
    return () => clearTimeout(t)
  }, [error])

  if (loading) {
    return <Spinner label="Loading tenants..." />
  }

  return (
    <div>
      <div className="card-header-row" style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600, margin: 0 }}>All Tenants</h2>
        <Link to="/tenants/new" className="btn btn-primary" style={{ width: 'auto', padding: '8px 16px' }}>
          Create Tenant
        </Link>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {!tenants || tenants.length === 0 ? (
        <EmptyState
          message="No tenants yet."
          action={<Link to="/tenants/new" className="btn btn-primary" style={{ width: 'auto' }}>Create your first tenant</Link>}
        />
      ) : (
        <div className="card">
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Slug</th>
                  <th>Name</th>
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
                    <td>
                      <Badge value={t.pii_config?.enabled ? 'On' : 'No rules'} />
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
