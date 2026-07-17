import { useEffect, useState } from 'react'

interface AuditEntry {
  id: number
  admin_username: string
  action: string
  target: string
  details: string
  created_at: string
}

export function AuditLog() {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/v1/audit', { credentials: 'include' })
      .then((r) => {
        if (!r.ok) throw new Error('fetch failed')
        return r.json()
      })
      .then((body) => {
        const d = body.data ?? body
        setEntries(Array.isArray(d.items) ? d.items : [])
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="card">
      <h3>Events (last 100)</h3>
      <div className="table-wrap">
        <table>
          <thead>
            <tr><th>Timestamp</th><th>Admin</th><th>Action</th><th>Target</th><th>Details</th></tr>
          </thead>
          <tbody>
            {entries.map((e) => (
              <tr key={e.id}>
                <td>{new Date(e.created_at).toLocaleString()}</td>
                <td>{e.admin_username}</td>
                <td><code>{e.action}</code></td>
                <td>{e.target}</td>
                <td>{e.details || '—'}</td>
              </tr>
            ))}
            {!loading && entries.length === 0 && <tr><td colSpan={5} className="text-muted" style={{ padding: 12 }}>No events</td></tr>}
            {loading && <tr><td colSpan={5} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  )
}
