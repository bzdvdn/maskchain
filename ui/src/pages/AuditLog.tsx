import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { useSort, sortRows } from '../hooks/useSort'
import { CopyButton } from '../components/CopyButton'
import { TimeRangePicker, type RangeValue } from '../components/TimeRangePicker'

interface AuditEntry {
  id: number
  admin_username: string
  action: string
  target: string
  details: string
  created_at: string
}

async function fetchAudit(): Promise<AuditEntry[]> {
  const res = await fetch('/api/v1/audit', { credentials: 'include' })
  if (!res.ok) throw new Error('fetch failed')
  const body = await res.json()
  const d = body.data ?? body
  return Array.isArray(d.items) ? d.items : []
}

type SortKey = keyof Pick<AuditEntry, 'created_at' | 'admin_username' | 'action' | 'target'>

export function AuditLog() {
  const { data: entries, loading } = useAsyncData<AuditEntry[]>(fetchAudit, [])
  const { key, dir, toggle } = useSort<AuditEntry>('created_at', 'desc')
  const [range, setRange] = useState<RangeValue>(() => {
    const to = new Date()
    const from = new Date(to.getTime() - 7 * 86400_000)
    return { mode: '7d', from: from.toISOString(), to: to.toISOString() }
  })
  const { mode, from, to } = range

  const inRange = useMemo(() => {
    const list = entries ?? []
    if (mode === 'all') return list
    const f = from ? new Date(from).getTime() : 0
    const t = to ? new Date(to).getTime() : Infinity
    return list.filter((e) => {
      const ts = new Date(e.created_at).getTime()
      return ts >= f && ts <= t
    })
  }, [entries, from, to, mode])

  const rows = useMemo(() => sortRows(inRange, key, dir), [inRange, key, dir])

  const th = (k: SortKey, label: string) => (
    <th>
      <SortHeader active={key === k} dir={dir} onClick={() => toggle(k)}>{label}</SortHeader>
    </th>
  )

  return (
    <div className="card">
      <div className="card-header-row">
        <h3>Events (last 100)</h3>
        <TimeRangePicker value={range} onChange={setRange} />
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {th('created_at', 'Timestamp')}
              {th('admin_username', 'Admin')}
              {th('action', 'Action')}
              {th('target', 'Target')}
              <th>Details</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((e) => (
              <tr key={e.id}>
                <td>{new Date(e.created_at).toLocaleString()}</td>
                <td>{e.admin_username}</td>
                <td><code>{e.action}</code></td>
                <td>{e.target}</td>
                <td>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                    {e.details || '—'}
                    {e.details && <CopyButton text={e.details} label="Copy details" />}
                  </span>
                </td>
              </tr>
            ))}
            {!loading && rows.length === 0 && <tr><td colSpan={5}><EmptyState message="No events" /></td></tr>}
            {loading && <tr><td colSpan={5}><TableSkeleton rows={4} cols={5} /></td></tr>}
          </tbody>
        </table>
      </div>
    </div>
  )
}
