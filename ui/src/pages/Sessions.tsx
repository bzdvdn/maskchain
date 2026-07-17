import { useEffect, useState } from 'react'

interface Session {
  session_id: string
  tenant_id: string
  model: string
  status: string
  created_at: string
  expires_at: string
  token_count: number
}

export function Sessions() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/v1/sessions', { credentials: 'include' })
      .then((r) => {
        if (!r.ok) throw new Error('fetch failed')
        return r.json()
      })
      .then((body) => {
        const d = body.data ?? body
        setSessions(Array.isArray(d.items) ? d.items : Array.isArray(d) ? d : [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  async function handleClose(sessionId: string) {
    await fetch(`/api/v1/sessions/${sessionId}`, { method: 'DELETE', credentials: 'include' })
    setSessions((prev) => prev.filter((s) => s.session_id !== sessionId))
  }

  async function handleExtend(sessionId: string) {
    await fetch(`/api/v1/sessions/${sessionId}/extend`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ttl_seconds: 1800 }),
      credentials: 'include',
    })
  }

  function fmtTime(t: string) {
    if (!t) return '—'
    return new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  return (
    <div className="card">
      <h3>Active Sessions</h3>
      <div className="table-wrap">
        <table>
          <thead>
            <tr><th>Session ID</th><th>Tenant</th><th>Model</th><th>Status</th><th>Created</th><th>Expires</th><th>Tokens Used</th><th></th></tr>
          </thead>
          <tbody>
            {sessions.map((s) => (
              <tr key={s.session_id}>
                <td><code>{s.session_id.slice(0, 12)}...</code></td>
                <td>{s.tenant_id}</td>
                <td>{s.model}</td>
                <td><span className={`badge ${s.status === 'active' ? 'badge-up' : s.status === 'expired' ? 'badge-down' : 'badge-warn'}`}>{s.status}</span></td>
                <td>{fmtTime(s.created_at)}</td>
                <td>{fmtTime(s.expires_at)}</td>
                <td>{s.token_count?.toLocaleString() ?? '—'}</td>
                <td>
                  {s.status === 'active' && (
                    <>
                      <button className="btn btn-small" onClick={() => handleExtend(s.session_id)}>Extend</button>
                      {' '}
                      <button className="btn btn-small btn-danger" onClick={() => handleClose(s.session_id)}>Close</button>
                    </>
                  )}
                </td>
              </tr>
            ))}
            {!loading && sessions.length === 0 && <tr><td colSpan={8} className="text-muted" style={{ padding: 12 }}>No sessions</td></tr>}
            {loading && <tr><td colSpan={8} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  )
}
