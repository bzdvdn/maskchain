import { useEffect, useState } from 'react'
import type {
  ConversationDetail,
  ConversationListItem,
} from '../api/conversations'
import {
  getConversation,
  listConversations,
} from '../api/conversations'
import { decodeBase64Utf8 } from '../utils/base64'

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
  const [items, setItems] = useState<ConversationListItem[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const perPage = 20
  const [loading, setLoading] = useState(true)
  const [detail, setDetail] = useState<ConversationDetail | null>(null)
  const [detailError, setDetailError] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    listConversations(page, perPage)
      .then((res) => {
        if (cancelled) return
        setItems(res.items)
        setTotal(res.pagination.total)
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [page, perPage])

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

  return (
    <div className="card">
      <h3>Conversations</h3>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Tenant</th>
              <th>Model</th>
              <th>Status</th>
              <th>Masked</th>
              <th>Streamed</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {items.map((c) => (
              <tr
                key={c.id}
                className={detail?.id === c.id ? 'row-active' : undefined}
                onClick={() => openDetail(c.id)}
                style={{ cursor: 'pointer' }}
              >
                <td><code>{c.id.slice(0, 8)}...</code></td>
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
            {!loading && items.length === 0 && (
              <tr><td colSpan={7} className="text-muted" style={{ padding: 12 }}>No conversations</td></tr>
            )}
            {loading && <tr><td colSpan={7} className="text-muted" style={{ padding: 12 }}>Loading...</td></tr>}
          </tbody>
        </table>
      </div>

      {total > perPage && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 12 }}>
          <button
            className="btn btn-small"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
          >
            Prev
          </button>
          <span className="text-muted">Page {page} / {totalPages} ({total})</span>
          <button
            className="btn btn-small"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
          >
            Next
          </button>
        </div>
      )}

      {detailError && <p className="text-muted" style={{ marginTop: 12 }}>Failed to load conversation detail</p>}

      {detail && (
        <div style={{ marginTop: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
            <h4>Conversation {detail.id}</h4>
            <button type="button" className="btn btn-small" onClick={() => setDetail(null)}>
              Collapse
            </button>
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
