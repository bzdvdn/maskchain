import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getIncident, type IncidentResponse } from '../../api/incidents'

// @sk-task 60-audit-incidents#T3.3: Incident detail with all fields (AC-005)
export function IncidentDetail() {
  const { id } = useParams<{ id: string }>()
  const [incident, setIncident] = useState<IncidentResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [notFound, setNotFound] = useState(false)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getIncident(id)
      .then(setIncident)
      .catch((err) => {
        if (err.message === 'Incident not found') setNotFound(true)
      })
      .finally(() => setLoading(false))
  }, [id])

  if (loading) return <div className="loading">Loading incident...</div>

  if (notFound || !incident) {
    return (
      <div className="not-found">
        <h1>Incident not found</h1>
        <p>The incident you are looking for does not exist.</p>
        <Link to="/incidents" className="btn">
          Back to list
        </Link>
      </div>
    )
  }

  return (
    <div className="incident-detail">
      <div className="page-header">
        <h1>Incident Detail</h1>
        <Link to="/incidents" className="btn">
          Back to list
        </Link>
      </div>

      <div className="detail-grid">
        <div className="detail-field">
          <label>ID</label>
          <span>{incident.id}</span>
        </div>
        <div className="detail-field">
          <label>Request ID</label>
          <span>{incident.request_id}</span>
        </div>
        <div className="detail-field">
          <label>Timestamp</label>
          <span>{new Date(incident.timestamp).toLocaleString()}</span>
        </div>
        <div className="detail-field">
          <label>Tenant</label>
          <span>{incident.tenant}</span>
        </div>
        <div className="detail-field">
          <label>Profile Slug</label>
          <span>{incident.profile_slug}</span>
        </div>
        <div className="detail-field">
          <label>Detector Type</label>
          <span>{incident.detector_type}</span>
        </div>
        <div className="detail-field">
          <label>Entry Value</label>
          <span>{incident.entry_value || '—'}</span>
        </div>
        <div className="detail-field">
          <label>Severity</label>
          <span className={`status-badge severity-${incident.severity}`}>
            {incident.severity}
          </span>
        </div>
        <div className="detail-field">
          <label>Action</label>
          <span>{incident.action}</span>
        </div>
      </div>

      {incident.prompt_snippet_redacted && (
        <section>
          <h2>Prompt Snippet (Redacted)</h2>
          <pre className="snippet-box">{incident.prompt_snippet_redacted}</pre>
        </section>
      )}

      {incident.response_snippet && (
        <section>
          <h2>Response Snippet</h2>
          <pre className="snippet-box">{incident.response_snippet}</pre>
        </section>
      )}
    </div>
  )
}
