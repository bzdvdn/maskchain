import { useEffect, useMemo, useState } from 'react'
import type { ConversationDetail, ConversationFilters, ConversationListItem } from '../api/conversations'
import {
  getConversation,
  listConversations,
} from '../api/conversations'
import { listTenants } from '../api/tenants'
import { decodeBase64Utf8 } from '../utils/base64'
import { AsyncSection, Button, Pagination, SortHeader, StatusPill, TableSkeleton } from '../components/ui'
import { CopyButton } from '../components/CopyButton'
import { useAsyncData } from '../hooks/useAsyncData'
import { useSort, sortRows } from '../hooks/useSort'
import { useWorkspace } from '../hooks/useWorkspace'
import { useUrlFilters } from '../hooks/useUrlFilters'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

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
  const url = useUrlFilters()
  const [page, setPage] = useState(() => url.getInt('page', 1))
  const perPage = 20
  const [workspace] = useWorkspace()
  const [filters, setFilters] = useState<ConversationFilters>(() => ({
    tenant_id: url.get('tenant') || workspace || undefined,
    status: url.get('status') || undefined,
    model: url.get('model') || undefined,
    masked: (url.get('masked') as ConversationFilters['masked']) || undefined,
  }))
  const [tenantOptions, setTenantOptions] = useState<string[]>([])
  const [detail, setDetail] = useState<ConversationDetail | null>(null)
  const [detailError, setDetailError] = useState(false)

  useEffect(() => {
    listTenants()
      .then((ts) => setTenantOptions(ts.map((t) => t.slug).sort()))
      .catch(() => {})
  }, [])

  useEffect(() => {
    setFilters((prev) => ({ ...prev, tenant_id: workspace || undefined }))
    setPage(1)
    url.set({ tenant: workspace || undefined, page: undefined })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace])

  const filterKey = useMemo(() => `${filters.tenant_id ?? ''}|${filters.status ?? ''}|${filters.model ?? ''}|${filters.masked ?? ''}`, [filters])
  const { data: result, loading, error, refetch } = useAsyncData(
    () => listConversations(page, perPage, filters),
    [page, perPage, filterKey],
  )
  const items = result?.items ?? []
  const total = result?.pagination.total ?? 0

  useAutoRefresh(refetch, 30)

  function setFilter<K extends keyof ConversationFilters>(key: K, value: string) {
    setFilters((prev) => ({ ...prev, [key]: (value || undefined) as ConversationFilters[K] | undefined }))
    setPage(1)
    const urlKey = key === 'tenant_id' ? 'tenant' : key
    url.set({ [urlKey]: value || undefined, page: undefined })
  }

  function changePage(p: number) {
    setPage(p)
    url.set({ page: p > 1 ? p : undefined })
  }
  const models = useMemo(() => Array.from(new Set(items.map((c) => c.model))).sort(), [items])
  const tenants = useMemo(
    () => Array.from(new Set([...tenantOptions, ...items.map((c) => c.tenant_id)])).sort(),
    [tenantOptions, items],
  )

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
    const d = new Date(t)
    if (Number.isNaN(d.getTime())) return '—'
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${pad(d.getHours())}:${pad(d.getMinutes())} ${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${String(d.getFullYear()).slice(2)}`
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
        <label className="filter-field">
          <span>Masked</span>
          <select value={filters.masked ?? ''} onChange={(e) => setFilter('masked', e.target.value)}>
            <option value="">All</option>
            <option value="true">Masked</option>
            <option value="false">Not masked</option>
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
          <AsyncSection
            as="tbody"
            colSpan={7}
            loading={loading}
            error={error}
            onRetry={refetch}
            empty={rows.length === 0}
            emptyMessage="No conversations"
            skeleton={<TableSkeleton rows={4} cols={7} />}
          >
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
                  <StatusPill tone={c.status === 'blocked' ? 'red' : c.status === 'error' ? 'amber' : 'green'}>
                    {c.status}
                  </StatusPill>
                </td>
                <td>{c.masked ? 'yes' : 'no'}</td>
                <td>{c.streamed ? 'yes' : 'no'}</td>
                <td>{fmtTime(c.created_at)}</td>
              </tr>
            ))}
          </AsyncSection>
        </table>
      </div>

      {total > perPage && (
        <Pagination page={page} totalPages={totalPages} total={total} onPage={changePage} />
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
