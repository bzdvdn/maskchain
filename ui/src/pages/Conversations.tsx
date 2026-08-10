import { useMemo, useState } from 'react'
import type { ConversationDetail, ConversationFilters, ConversationListItem } from '../api/conversations'
import {
  getConversation,
  listConversations,
} from '../api/conversations'
import { decodeBase64Utf8 } from '../utils/base64'
import { Button, Pagination, SortHeader, TableSkeleton } from '../components/ui'
import { CopyButton } from '../components/CopyButton'
import { useAsyncData } from '../hooks/useAsyncData'
import { useSort, sortRows } from '../hooks/useSort'

interface MaskGroup {
  original: string
  placeholders: string[]
}

// @sk-task conversation-logging#T3.2: groupMasking builds original -> placeholder mapping (AC-005)
export function groupMasking(maskingB64: string | null): MaskGroup[] {
  if (!maskingB64) return []
  let entries: { placeholder: string; original: string }[]
  try {
    entries = JSON.parse(decodeBase64Utf8(maskingB64))
  } catch {
    return []
  }
  const byOriginal = new Map<string, string[]>()
  for (const e of entries) {
    if (!e.original) continue
    const list = byOriginal.get(e.original) ?? []
    list.push(e.placeholder)
    byOriginal.set(e.original, list)
  }
  const groups: MaskGroup[] = []
  for (const [original, placeholders] of byOriginal.entries()) {
    groups.push({ original, placeholders })
  }
  return groups
}

// @sk-task conversation-logging#T3.2: Conversations page lists records and shows decoded detail (AC-005, AC-006)
export function Conversations() {
  const [page, setPage] = useState(1)
  const perPage = 20
  const [filters, setFilters] = useState<ConversationFilters>({})
  const [detail, setDetail] = useState<ConversationDetail | null>(null)
  const [detailError, setDetailError] = useState(false)

  const filterKey = useMemo(() => `${filters.tenant_id ?? ''}|${filters.status ?? ''}|${filters.model ?? ''}`, [filters])
  const { data: result, loading } = useAsyncData(
    () => listConversations(page, perPage, filters),
    [page, perPage, filterKey],
  )
  const items = result?.items ?? []
  const total = result?.pagination.total ?? 0

  function setFilter(key: keyof ConversationFilters, value: string) {
    setFilters((prev) => ({ ...prev, [key]: value || undefined }))
    setPage(1)
  }

  const models = useMemo(() => Array.from(new Set(items.map((c) => c.model))).sort(), [items])
  const tenants = useMemo(() => Array.from(new Set(items.map((c) => c.tenant_id))).sort(), [items])

  async function openDetail(id: string) {
    if (detail?.id === id) {
      setDetail(null)
      return
    }
    setDetailError(false)
    try {
      const d = await getConversation(id)
      setDetail(d)
    } catch {
      setDetailError(true)
      setDetail(null)
    }
  }

  function fmtTime(t: string) {
    if (!t) return '—'
    return new Date(t).toLocaleString([], {
      year: '2-digit',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const totalPages = Math.max(1, Math.ceil(total / perPage))
  const masking = detail ? groupMasking(detail.payload.masking) : []

  type SortKey = keyof ConversationListItem
  const sort = useSort<ConversationListItem>('created_at', 'desc')
  const rows = useMemo(() => sortRows(items, sort.key, sort.dir), [items, sort])
  const th = (k: SortKey, label: string) => (
    <th>
      <SortHeader active={sort.key === k} dir={sort.dir} onClick={() => sort.toggle(k)}>{label}</SortHeader>
    </th>
  )

  return (
    <div className="card">
      <h3>Conversations</h3>
      <div className="filter-bar" role="search">
        <label className="filter-field">
          <span>Tenant</span>
          <select value={filters.tenant_id ?? ''} onChange={(e) => setFilter('tenant_id', e.target.value)}>
            <option value="">All tenants</option>
            {tenants.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </label>
        <label className="filter-field">
          <span>Status</span>
          <select value={filters.status ?? ''} onChange={(e) => setFilter('status', e.target.value)}>
            <option value="">All statuses</option>
            <option value="ok">ok</option>
            <option value="error">error</option>
            <option value="blocked">blocked</option>
          </select>
        </label>
        <label className="filter-field">
          <span>Model</span>
          <select value={filters.model ?? ''} onChange={(e) => setFilter('model', e.target.value)}>
            <option value="">All models</option>
            {models.map((m) => <option key={m} value={m}>{m}</option>)}
          </select>
        </label>
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              {th('id', 'ID')}
              {th('tenant_id', 'Tenant')}
              {th('model', 'Model')}
              {th('status', 'Status')}
              {th('masked', 'Masked')}
              {th('streamed', 'Streamed')}
              {th('created_at', 'Created')}
            </tr>
          </thead>
          <tbody>
            {rows.map((c) => (
              <tr
                key={c.id}
                className={detail?.id === c.id ? 'row-active' : undefined}
                onClick={() => openDetail(c.id)}
                style={{ cursor: 'pointer' }}
              >
                <td>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                    <code>{c.id.slice(0, 8)}...</code>
                    <CopyButton text={c.id} label="Copy ID" />
                  </span>
                </td>
                <td>{c.tenant_id}</td>
                <td>{c.model}</td>
                <td>
                  <span className={`badge ${c.status === 'blocked' ? 'badge-down' : c.status === 'error' ? 'badge-warn' : 'badge-up'}`}>
                    {c.status}
                  </span>
                </td>
                <td>{c.masked ? 'yes' : 'no'}</td>
                <td>{c.streamed ? 'yes' : 'no'}</td>
                <td>{fmtTime(c.created_at)}</td>
              </tr>
            ))}
            {!loading && rows.length === 0 && (
              <tr><td colSpan={7} className="text-muted" style={{ padding: 12 }}>No conversations</td></tr>
            )}
            {loading && <tr><td colSpan={7}><TableSkeleton rows={4} cols={7} /></td></tr>}
          </tbody>
        </table>
      </div>

      {total > perPage && (
        <Pagination page={page} totalPages={totalPages} total={total} onPage={(p) => setPage(p)} />
      )}

      {detailError && <p className="text-muted" style={{ marginTop: 12 }}>Failed to load conversation detail</p>}

      {detail && (
        <div style={{ marginTop: 16 }}>
          <div className="card-header-row">
            <h4 style={{ margin: 0 }}>Conversation {detail.id}</h4>
            <Button size="small" onClick={() => setDetail(null)}>Collapse</Button>
          </div>
          {detail.mask_id && (
            <p className="text-muted" style={{ fontSize: 12 }}>
              Mask ID: <code>{detail.mask_id}</code>
            </p>
          )}

          {masking.length > 0 && (
            <div className="card" style={{ marginTop: 8 }}>
              <h4>Mask applied</h4>
              <table>
                <thead>
                  <tr><th>Original</th><th>Placeholders</th></tr>
                </thead>
                <tbody>
                  {masking.map((g) => (
                    <tr key={g.original}>
                      <td>{g.original}</td>
                      <td><code>{g.placeholders.join(', ')}</code></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="card" style={{ marginTop: 8 }}>
            <h4>Request</h4>
            <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{decodeBase64Utf8(detail.payload.request)}</pre>
          </div>

          {detail.payload.response && (
            <div className="card" style={{ marginTop: 8 }}>
              <h4>Response {detail.streamed ? '(stream)' : ''}</h4>
              <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{decodeBase64Utf8(detail.payload.response)}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
