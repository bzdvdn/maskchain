import { useCallback, useEffect, useState } from 'react'
import { RefreshCw } from 'lucide-react'
import { StatusPill, type StatusTone } from '../components/ui'
import { getSystemStatus, type SystemStatus } from '../api/admin'

function humanUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function healthTone(status: string): StatusTone {
  if (status === 'ok' || status === 'up') return 'green'
  if (status === 'down') return 'red'
  if (status === 'degraded') return 'amber'
  return 'gray'
}

export function Settings() {
  const [status, setStatus] = useState<SystemStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setStatus(await getSystemStatus())
      setError('')
    } catch {
      setError('Failed to load system status')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  if (loading && !status) return <div className="loading">Loading system status…</div>

  return (
    <div>
      <div className="card">
        <div className="card-header-row">
          <h3>Server</h3>
          <button className="btn btn-small" onClick={load}><RefreshCw size={13} /> Refresh</button>
        </div>
        {error && <p className="error-banner">{error}</p>}
        {status && (
          <div className="kv">
            <div className="k">Version</div><div className="v mono">{status.version}</div>
            <div className="k">Uptime</div><div className="v">{humanUptime(status.uptime_seconds)}</div>
            <div className="k">At-rest encryption</div>
            <div className="v">
              {status.key_at_rest?.configured
                ? <StatusPill tone="green">configured</StatusPill>
                : <StatusPill tone="gray">not configured</StatusPill>}
              {status.key_at_rest?.cipher && <span className="muted u-ml8">{status.key_at_rest.cipher}</span>}
            </div>
            <div className="k">Aggregated health</div>
            <div className="v"><StatusPill tone={healthTone(status.health?.status ?? 'unknown')}>{status.health?.status ?? 'unknown'}</StatusPill></div>
          </div>
        )}
      </div>

      {status && status.health?.checks && (
        <div className="card">
          <div className="card-header-row"><h3>Stores &amp; dependencies</h3></div>
          <div className="table-wrap">
            <table className="tbl">
              <thead><tr><th>Component</th><th>Status</th><th className="num">Latency</th><th>Detail</th></tr></thead>
              <tbody>
                {Object.entries(status.health.checks).map(([name, check]) => (
                  <tr key={name}>
                    <td className="u-fw">{name}</td>
                    <td><StatusPill tone={healthTone(check.status)}>{check.status}</StatusPill></td>
                    <td className="num">{check.latency_ms} ms</td>
                    <td className="muted">{check.error ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {status && (
        <div className="card">
          <div className="card-header-row"><h3>Configuration</h3></div>
          {status.config_diff?.watched ? (
            <>
              {status.config_diff.sections && status.config_diff.sections.length > 0 ? (
                <>
                  <div className="banner amber">
                    <RefreshCw size={15} />
                    <div className="text"><b>Live config differs from the file.</b> Sections: {status.config_diff.sections.join(', ')}. The config watcher applies file changes automatically on disk change.</div>
                  </div>
                  <div className="u-wrap">
                    {status.config_diff.sections.map((s) => <span key={s} className="chip warn">{s}</span>)}
                  </div>
                </>
              ) : (
                <div className="empty-state u-center">Config is in sync — no live/full file differences detected.</div>
              )}
            </>
          ) : (
            <div className="muted-lg">Config watch is disabled — config changes apply on restart.</div>
          )}
        </div>
      )}
    </div>
  )
}