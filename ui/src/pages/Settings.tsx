export function Settings() {
  return (
    <div className="stats-grid" style={{ gridTemplateColumns: '1fr 1fr' }}>
      <div className="card">
        <h3>Server</h3>
        <table>
          <tbody>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Port</td><td style={{ padding: '8px 12px' }}><code>8081</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Log Level</td><td style={{ padding: '8px 12px' }}><span className="badge badge-warn">debug</span></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Shutdown Timeout</td><td style={{ padding: '8px 12px' }}><code>10s</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Tenant Reload</td><td style={{ padding: '8px 12px' }}><code>15s</code></td></tr>
          </tbody>
        </table>
      </div>
      <div className="card">
        <h3>Admin</h3>
        <table>
          <tbody>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Username</td><td style={{ padding: '8px 12px' }}><code>admin</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Session TTL</td><td style={{ padding: '8px 12px' }}><code>30m</code></td></tr>
            <tr><td style={{ fontWeight: 600, padding: '8px 12px' }}>Debug Enabled</td><td style={{ padding: '8px 12px' }}><span className="badge badge-up">true</span></td></tr>
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
