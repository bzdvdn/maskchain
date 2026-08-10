import { useEffect, useMemo, useState } from 'react'
import { useAsyncData } from '../hooks/useAsyncData'
import { Badge, Button, EmptyState, SortHeader, TableSkeleton } from '../components/ui'
import { useToast } from '../components/Toast'
import { useSort, sortRows } from '../hooks/useSort'
import { TimeRangePicker, type RangeValue } from '../components/TimeRangePicker'
import { ConfirmModal } from '../components/ConfirmModal'
import { CopyButton } from '../components/CopyButton'

interface Session {
  session_id: string
  tenant_id: string
  model: string
  status: string
  created_at: string
  expires_at: string
  token_count: number
}

async function fetchSessions(): Promise<Session[]> {
  const res = await fetch('/api/v1/sessions', { credentials: 'include' })
  if (!res.ok) throw new Error('fetch failed')
  const body = await res.json()
  const d = body.data ?? body
  return Array.isArray(d.items) ? d.items : Array.isArray(d) ? d : []
}

type SortKey = keyof Pick<Session, 'session_id' | 'tenant_id' | 'model' | 'status' | 'created_at' | 'token_count'>

export function Sessions() {
  const { data: sessions, loading, refetch } = useAsyncData<Session[]>(fetchSessions, [])
  const { toast } = useToast()
  const { key, dir, toggle } = useSort<Session>('created_at', 'desc')
  const [refreshSec, setRefreshSec] = useState(10)
  const [asOf, setAsOf] = useState<Date | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [confirmTarget, setConfirmTarget] = useState<string | null>(null)
  const [closing, setClosing] = useState(false)
  const [range, setRange] = useState<RangeValue>(() => {
    const to = new Date()
    const from = new Date(to.getTime() - 7 * 86400_000)
    return { mode: '7d', from: from.toISOString(), to: to.toISOString() }
  })
  const { mode, from, to } = range

  const inRange = useMemo(() => {
    const list = sessions ?? []
    if (mode === 'all') return list
    const f = from ? new Date(from).getTime() : 0
    const t = to ? new Date(to).getTime() : Infinity
    return list.filter((s) => {
      const ts = new Date(s.created_at).getTime()
      return ts >= f && ts <= t
    })
  }, [sessions, from, to, mode])

  const rows = useMemo(() => sortRows(inRange, key, dir), [inRange, key, dir])

  useEffect(() => {
    if (refreshSec <= 0) return
    const interval = setInterval(() => {
      refetch()
      setAsOf(new Date())
    }, refreshSec * 1_000)
    return () => clearInterval(interval)
  }, [refreshSec, refetch])

  async function execClose(ids: string[]) {
    setClosing(true)
    try {
      await Promise.all(ids.map((id) => fetch(`/api/v1/sessions/${id}`, { method: 'DELETE', credentials: 'include' })))
      toast(ids.length === 1 ? 'Session closed' : `${ids.length} sessions closed`, 'success')
      setSelected((prev) => {
        const next = new Set(prev)
        for (const id of ids) next.delete(id)
        return next
      })
      refetch()
    } catch {
      toast('Failed to close session(s)', 'error')
    } finally {
      setClosing(false)
    }
  }

  function handleClose(sessionId: string) {
    setConfirmTarget(sessionId)
  }

  function handleBulkClose() {
    setConfirmTarget('__bulk__')
  }

  function confirmAction() {
    if (confirmTarget === '__bulk__') {
      execClose(Array.from(selected))
    } else if (confirmTarget) {
      execClose([confirmTarget])
    }
    setConfirmTarget(null)
  }

  function toggleSelect(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function toggleSelectAll() {
    setSelected((prev) => (prev.size === rows.length ? new Set() : new Set(rows.map((r) => r.session_id))))
  }

  async function handleExtend(sessionId: string) {
    try {
      await fetch(`/api/v1/sessions/${sessionId}/extend`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ttl_seconds: 1800 }),
        credentials: 'include',
      })
      toast('Session extended by 30m', 'success')
      refetch()
    } catch {
      toast('Failed to extend session', 'error')
    }
  }

  function fmtTime(t: string) {
    if (!t) return '—'
    return new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  const th = (k: SortKey, label: string, num = false) => (
    <th className={num ? 'num' : undefined}>
      <SortHeader active={key === k} dir={dir} onClick={() => toggle(k)}>{label}</SortHeader>
    </th>
  )

  return (
    <div className="card">
      <div className="card-header-row">
        <h3 style={{ margin: 0 }}>Active Sessions</h3>
        <div className="header-actions" style={{ gap: 8 }}>
          <span className="text-muted" style={{ fontSize: 12 }}>
            {refreshSec > 0 ? `Auto-refresh ${refreshSec}s` : 'Auto-refresh off'}
            {asOf && refreshSec > 0 ? ` · ${asOf.toLocaleTimeString()}` : ''}
          </span>
          {[0, 5, 10, 30].map((s) => (
            <Button
              key={s}
              size="small"
              onClick={() => setRefreshSec(s)}
              style={refreshSec === s ? { borderColor: 'var(--accent)', color: 'var(--accent)' } : undefined}
            >
              {s === 0 ? 'Off' : `${s}s`}
            </Button>
          ))}
          <Button size="small" onClick={() => refetch()}>Refresh</Button>
          {selected.size > 0 && (
            <Button size="small" variant="danger" onClick={handleBulkClose}>Close {selected.size} selected</Button>
          )}
          <TimeRangePicker value={range} onChange={setRange} />
        </div>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th style={{ width: 32 }}>
                <input
                  type="checkbox"
                  aria-label="Select all sessions"
                  checked={selected.size > 0 && selected.size === rows.length}
                  onChange={toggleSelectAll}
                  disabled={rows.length === 0}
                />
              </th>
              {th('session_id', 'Session ID')}
              {th('tenant_id', 'Tenant')}
              {th('model', 'Model')}
              {th('status', 'Status')}
              {th('created_at', 'Created')}
              <th>Expires</th>
              {th('token_count', 'Tokens Used', true)}
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((s) => (
              <tr key={s.session_id}>
                <td>
                  <input
                    type="checkbox"
                    aria-label={`Select ${s.session_id}`}
                    checked={selected.has(s.session_id)}
                    onChange={() => toggleSelect(s.session_id)}
                  />
                </td>
                <td>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                    <code>{s.session_id.slice(0, 12)}...</code>
                    <CopyButton text={s.session_id} label="Copy session ID" />
                  </span>
                </td>
                <td>{s.tenant_id}</td>
                <td>{s.model}</td>
                <td><Badge value={s.status} /></td>
                <td>{fmtTime(s.created_at)}</td>
                <td>{fmtTime(s.expires_at)}</td>
                <td className="num">{s.token_count?.toLocaleString() ?? '—'}</td>
                <td>
                  {s.status === 'active' && (
                    <div className="header-actions">
                      <Button size="small" onClick={() => handleExtend(s.session_id)}>Extend</Button>
                      <Button size="small" variant="danger" onClick={() => handleClose(s.session_id)}>Close</Button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
            {!loading && rows.length === 0 && <tr><td colSpan={9}><EmptyState message="No sessions" /></td></tr>}
            {loading && <tr><td colSpan={9}><TableSkeleton rows={4} cols={9} /></td></tr>}
          </tbody>
        </table>
      </div>

      <ConfirmModal
        open={confirmTarget !== null}
        title={confirmTarget === '__bulk__' ? `Close ${selected.size} sessions` : 'Close session'}
        message={confirmTarget === '__bulk__'
          ? `This will close ${selected.size} selected session(s). Continue?`
          : 'This session will be closed immediately. Continue?'}
        confirmLabel="Close"
        busy={closing}
        onConfirm={confirmAction}
        onCancel={() => setConfirmTarget(null)}
      />
    </div>
  )
}
