import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import {
  getProfile,
  deleteProfile,
  type ProfileResponse,
} from '../../api/profiles'
import type { DictionaryDTO, PreprocessorDef } from '../../api/profiles'

// @sk-task 41-profiles-ui#T3.1: Profile detail with all fields + 404 state (AC-003)
export function ProfileDetail() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const [profile, setProfile] = useState<ProfileResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)

  useEffect(() => {
    if (!slug) return
    setLoading(true)
    getProfile(slug)
      .then(setProfile)
      .catch((err) => {
        if (err.name === 'NotFoundError') setNotFound(true)
      })
      .finally(() => setLoading(false))
  }, [slug])

  async function handleDelete() {
    if (!slug) return
    setDeleting(true)
    try {
      await deleteProfile(slug)
      navigate('/profiles')
    } catch {
      setDeleting(false)
      setShowConfirm(false)
    }
  }

  if (loading) return <div className="loading">Loading profile...</div>

  if (notFound || !profile) {
    return (
      <div className="not-found">
        <h1>Profile not found</h1>
        <p>The profile you are looking for does not exist.</p>
        <Link to="/profiles" className="btn">
          Back to list
        </Link>
      </div>
    )
  }

  return (
    <div className="profile-detail">
      <div className="page-header">
        <h1>{profile.name}</h1>
        <div className="header-actions">
          <Link to={`/profiles/${profile.slug}/edit`} className="btn">
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

      {showConfirm && (
        <div className="confirm-dialog">
          <p>
            Are you sure you want to delete profile "{profile.name}"? This
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

      <div className="detail-grid">
        <div className="detail-field">
          <label>Slug</label>
          <span>{profile.slug}</span>
        </div>
        <div className="detail-field">
          <label>Status</label>
          <span className={`status-badge status-${profile.status}`}>
            {profile.status}
          </span>
        </div>
        {profile.description && (
          <div className="detail-field">
            <label>Description</label>
            <span>{profile.description}</span>
          </div>
        )}
      </div>

      {profile.dictionaries && profile.dictionaries.length > 0 && (
        <section>
          <h2>Dictionaries</h2>
          {profile.dictionaries.map((d: DictionaryDTO, i: number) => (
            <div key={i} className="card">
              <h3>{d.name}</h3>
              <p>Match mode: {d.match_mode}</p>
              <ul>
                {d.entries.map((e: string, j: number) => (
                  <li key={j}>{e}</li>
                ))}
              </ul>
            </div>
          ))}
        </section>
      )}

      {profile.preprocessors && profile.preprocessors.length > 0 && (
        <section>
          <h2>Preprocessors</h2>
          {profile.preprocessors.map((pp: PreprocessorDef, i: number) => (
            <div key={i} className="card">
              <h3>{pp.name}</h3>
              <p>Type: {pp.type}</p>
              <ul>
                {pp.rules.map((r: { columns?: string[]; path?: string; mask: 'full' | 'surname' }, j: number) => (
                  <li key={j}>
                    {r.columns?.join(', ')}
                    {r.path ? ` (path: ${r.path})` : ''} → {r.mask}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>
      )}
    </div>
  )
}
