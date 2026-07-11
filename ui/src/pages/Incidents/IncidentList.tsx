import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  listIncidents,
  exportIncidents,
  type IncidentResponse,
  type IncidentFilterParams,
} from '../../api/incidents'

// @sk-task 60-audit-incidents#T3.2: Incident list with filters and pagination (AC-005)
export function IncidentList() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [incidents, setIncidents] = useState<IncidentResponse[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const severity = searchParams.get('severity') || ''
  const tenant = searchParams.get('tenant') || ''
  const profileSlug = searchParams.get('profile_slug') || ''
  const page = parseInt(searchParams.get('page') || '1', 10)
  const pageSize = 20

  useEffect(() => {
    setLoading(true)
    setError(null)

    const params: IncidentFilterParams = { page, page_size: pageSize }
    if (severity) params.severity = severity
    if (tenant) params.tenant = tenant
    if (profileSlug) params.profile_slug = profileSlug

    listIncidents(params)
      .then((res) => {
        setIncidents(res.data)
        setTotal(res.total)
      })
      .catch(() => setError('Failed to load incidents. Please try again.'))
      .finally(() => setLoading(false))
  }, [severity, tenant, profileSlug, page])

  function updateFilter(key: string, value: string) {
    const next = new URLSearchParams(searchParams)
    if (value) {
      next.set(key, value)
    } else {
      next.delete(key)
    }
    next.set('page', '1')
    setSearchParams(next)
  }

  function goToPage(p: number) {
    const next = new URLSearchParams(searchParams)
    next.set('page', String(p))
    setSearchParams(next)
  }

  async function handleExport(format: 'csv' | 'json') {
    try {
      const params: IncidentFilterParams & { format: 'csv' | 'json' } = { format }
      if (severity) params.severity = severity
      if (tenant) params.tenant = tenant
      if (profileSlug) params.profile_slug = profileSlug

      const blob = await exportIncidents(params)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `incidents.${format}`
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      setError('Failed to export incidents.')
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <div className="incident-list">
      <div className="page-header">
        <h1>Incidents</h1>
        <div className="header-actions">
          <button type="button" className="btn" onClick={() => handleExport('csv')}>
            Export CSV
          </button>
          <button type="button" className="btn" onClick={() => handleExport('json')}>
            Export JSON
          </button>
        </div>
      </div>

      <div className="filters">
        <input
          type="text"
          placeholder="Filter by severity"
          value={severity}
          onChange={(e) => updateFilter('severity', e.target.value)}
        />
        <input
          type="text"
          placeholder="Filter by tenant"
          value={tenant}
          onChange={(e) => updateFilter('tenant', e.target.value)}
        />
        <input
          type="text"
          placeholder="Filter by profile slug"
          value={profileSlug}
          onChange={(e) => updateFilter('profile_slug', e.target.value)}
        />
      </div>

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="loading">Loading incidents...</div>
      ) : incidents.length === 0 ? (
        <div className="empty-state">
          <p>No incidents found.</p>
        </div>
      ) : (
        <>
          <table className="incidents-table">
            <thead>
              <tr>
                <th>Timestamp</th>
                <th>Severity</th>
                <th>Tenant</th>
                <th>Profile Slug</th>
                <th>Detector Type</th>
                <th>Action</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {incidents.map((inc) => (
                <tr key={inc.id}>
                  <td>{new Date(inc.timestamp).toLocaleString()}</td>
                  <td>
                    <span className={`status-badge severity-${inc.severity}`}>
                      {inc.severity}
                    </span>
                  </td>
                  <td>{inc.tenant}</td>
                  <td>{inc.profile_slug}</td>
                  <td>{inc.detector_type}</td>
                  <td>{inc.action}</td>
                  <td>
                    <Link to={`/incidents/${inc.id}`}>View</Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="pagination">
            <button
              type="button"
              disabled={page <= 1}
              onClick={() => goToPage(page - 1)}
            >
              Previous
            </button>
            <span>
              Page {page} of {totalPages} ({total} total)
            </span>
            <button
              type="button"
              disabled={page >= totalPages}
              onClick={() => goToPage(page + 1)}
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  )
}
