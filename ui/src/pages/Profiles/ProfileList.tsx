import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listProfiles, type ProfileListItem } from '../../api/profiles'

// @sk-task 41-profiles-ui#T2.2: Profile list with paginated table (AC-002)
export function ProfileList() {
  const [profiles, setProfiles] = useState<ProfileListItem[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const pageSize = 20

  useEffect(() => {
    setLoading(true)
    setError(null)
    listProfiles(page, pageSize)
      .then((res) => {
        setProfiles(res.data)
        setTotal(res.total)
      })
      .catch(() => setError('Failed to load profiles. Please try again.'))
      .finally(() => setLoading(false))
  }, [page])

  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  if (error) {
    return <div className="error-banner">{error}</div>
  }

  if (loading) {
    return <div className="loading">Loading profiles...</div>
  }

  return (
    <div className="profile-list">
      <div className="page-header">
        <h1>Profiles</h1>
        <Link to="/profiles/new" className="btn btn-primary">
          Create Profile
        </Link>
      </div>

      {profiles.length === 0 ? (
        <div className="empty-state">
          <p>No profiles yet.</p>
          <Link to="/profiles/new" className="btn btn-primary">
            Create your first profile
          </Link>
        </div>
      ) : (
        <>
          <table className="profiles-table">
            <thead>
              <tr>
                <th>Slug</th>
                <th>Name</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {profiles.map((p) => (
                <tr key={p.slug}>
                  <td>{p.slug}</td>
                  <td>{p.name}</td>
                  <td>
                    <span className={`status-badge status-${p.status}`}>
                      {p.status}
                    </span>
                  </td>
                  <td>
                    <Link to={`/profiles/${p.slug}`} className="btn btn-small">
                      View
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {totalPages > 1 && (
            <div className="pagination">
              <button
                disabled={page <= 1}
                onClick={() => setPage((p) => p - 1)}
              >
                Previous
              </button>
              <span>
                Page {page} of {totalPages}
              </span>
              <button
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
