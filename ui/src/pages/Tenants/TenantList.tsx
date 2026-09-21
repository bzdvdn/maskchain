import { useState } from 'react'
import { Link } from 'react-router-dom'
import { listTenantsPage, type TenantListItem } from '../../api/tenants'
import { AsyncSection, Pagination, Spinner, StatusPill, statusTone } from '../../components/ui'
import { useAsyncData } from '../../hooks/useAsyncData'

const PER_PAGE = 20

export function TenantList() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState('')

  const { data, loading, error, refetch } = useAsyncData(
    () => listTenantsPage({ page, perPage: PER_PAGE, search }),
    [page, search],
  )
  const tenants = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))

  function onSearch(value: string) {
    setSearch(value)
    setPage(1)
  }

  return (
    <div>
      <div className="card-header-row" style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 16, fontWeight: 600, margin: 0 }}>All Tenants</h2>
        <Link to="/tenants/new" className="btn btn-primary" style={{ width: 'auto', padding: '8px 16px' }}>
          Create Tenant
        </Link>
      </div>

      <div className="toolbar">
        <div className="search">
          <input
            placeholder="Search by slug or name…"
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            aria-label="Search tenants"
          />
        </div>
      </div>

      <AsyncSection
        loading={loading}
        error={error}
        onRetry={refetch}
        empty={tenants.length === 0}
        emptyMessage={search ? 'No tenants match your search.' : 'No tenants yet.'}
        emptyAction={<Link to="/tenants/new" className="btn btn-primary" style={{ width: 'auto' }}>Create your first tenant</Link>}
        skeleton={<Spinner label="Loading tenants..." />}
      >
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
                {tenants.map((t: TenantListItem) => (
                  <tr key={t.slug}>
                    <td><code>{t.slug}</code></td>
                    <td>{t.name}</td>
                    <td>
                      <StatusPill tone={statusTone(t.pii_config?.enabled ? 'On' : 'No rules')} withDot={false}>
                        {t.pii_config?.enabled ? 'On' : 'No rules'}
                      </StatusPill>
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
      </AsyncSection>

      {total > PER_PAGE && (
        <Pagination page={page} totalPages={totalPages} total={total} onPage={setPage} />
      )}
    </div>
  )
}
