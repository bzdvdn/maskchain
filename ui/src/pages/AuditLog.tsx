import { useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { useUrlFilters } from '../hooks/useUrlFilters'
import { AsyncSection, Card, SortHeader, TableSkeleton } from '../components/ui'
import { useSort, sortRows } from '../hooks/useSort'
import { CopyButton } from '../components/CopyButton'
import { TimeRangePicker, type RangeValue } from '../components/TimeRangePicker'
import { listAudit, type AuditEntry } from '../api/audit'

async function fetchAudit(): Promise<AuditEntry[]> {
  return listAudit(100)
}

type SortKey = keyof Pick<AuditEntry, 'created_at' | 'admin_username' | 'action' | 'target'>

export function AuditLog() {
  const { data: entries, loading, error, refetch } = useAsyncData<AuditEntry[]>(fetchAudit, [])
  const { key, dir, toggle } = useSort<AuditEntry>('created_at', 'desc')
  const url = useUrlFilters()
  const [range, setRange] = useState<RangeValue>(() => {
    const to = new Date()
    const from = new Date(to.getTime() - 7 * 86400_000)
    return { mode: (url.get('range') as RangeValue['mode']) || '7d', from: from.toISOString(), to: to.toISOString() }
  })

  function changeRange(next: RangeValue) {
    setRange(next)
    url.set({ range: next.mode === '7d' ? undefined : next.mode })
  }
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
    <Card>
      <div className="card-header-row">
        <h3>Events (last 100)</h3>
        <TimeRangePicker value={range} onChange={changeRange} />
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
          <AsyncSection
            as="tbody"
            colSpan={5}
            loading={loading}
            error={error}
            onRetry={refetch}
            empty={rows.length === 0}
            emptyMessage="No events"
            skeleton={<TableSkeleton rows={4} cols={5} />}
          >
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
          </AsyncSection>
        </table>
      </div>
    </Card>
  )
}
