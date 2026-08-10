export function Settings() {
  return (
    <div className="stats-grid" style={{ gridTemplateColumns: '1fr 1fr' }}>
      <div className="card">
        <h3>Server</h3>
        <table>
          <tbody>
            <tr><td className="kv-label">Port</td><td className="kv-value"><code>8081</code></td></tr>
            <tr><td className="kv-label">Log Level</td><td className="kv-value"><span className="badge badge-warn">debug</span></td></tr>
            <tr><td className="kv-label">Shutdown Timeout</td><td className="kv-value"><code>10s</code></td></tr>
            <tr><td className="kv-label">Tenant Reload</td><td className="kv-value"><code>15s</code></td></tr>
          </tbody>
        </table>
      </div>
      <div className="card">
        <h3>Admin</h3>
        <table>
          <tbody>
            <tr><td className="kv-label">Username</td><td className="kv-value"><code>admin</code></td></tr>
            <tr><td className="kv-label">Session TTL</td><td className="kv-value"><code>30m</code></td></tr>
            <tr><td className="kv-label">Debug Enabled</td><td className="kv-value"><span className="badge badge-up">true</span></td></tr>
          </tbody>
        </table>
      </div>
      <div className="card">
        <h3>Database</h3>
        <div className="table-wrap">
          <table>
            <thead>
              <tr><th>Type</th><th>Host</th><th>Pool</th><th>Status</th></tr>
            </thead>
            <tbody>
              <tr><td>PostgreSQL</td><td><code>postgres:5432</code></td><td>10/25 conns</td><td><span className="badge badge-up">Connected</span></td></tr>
              <tr><td>Valkey</td><td><code>valkey:6379</code></td><td>5/10 conns</td><td><span className="badge badge-up">Connected</span></td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
