import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  LayoutDashboard,
  ChartColumn,
  Users,
  Radio,
  MessageSquare,
  Route,
  ScrollText,
  Settings,
  FileJson,
  Search,
  CornerDownLeft,
  KeyRound,
  Wallet,
} from 'lucide-react'
import { listTenants } from '../api/tenants'
import { listConversations } from '../api/conversations'

interface NavEntry {
  to: string
  label: string
  keywords: string
  icon: typeof LayoutDashboard
}

const NAV: NavEntry[] = [
  { to: '/', label: 'Dashboard', keywords: 'dashboard home overview', icon: LayoutDashboard },
  { to: '/analytics', label: 'Analytics', keywords: 'analytics usage tokens cost charts', icon: ChartColumn },
  { to: '/tenants', label: 'Tenants', keywords: 'tenants create list', icon: Users },
  { to: '/sessions', label: 'Sessions', keywords: 'sessions active live', icon: Radio },
  { to: '/conversations', label: 'Conversations', keywords: 'conversations messages chat', icon: MessageSquare },
  { to: '/routing', label: 'Routing', keywords: 'routing providers rules models', icon: Route },
  { to: '/keys', label: 'Keys', keywords: 'keys api virtual scoped models access', icon: KeyRound },
  { to: '/budgets', label: 'Budgets', keywords: 'budgets spend limits cap cost enforce', icon: Wallet },
  { to: '/audit', label: 'Audit Log', keywords: 'audit log events admin', icon: ScrollText },
  { to: '/settings', label: 'Settings', keywords: 'settings config', icon: Settings },
  { to: '/swagger', label: 'Swagger', keywords: 'swagger api docs openapi', icon: FileJson },
]

interface Result {
  key: string
  group: string
  label: string
  hint?: string
  icon: typeof LayoutDashboard
  to: string
}

function matches(q: string, ...fields: string[]): boolean {
  if (!q) return true
  const ql = q.toLowerCase()
  return fields.some((f) => f.toLowerCase().includes(ql))
}

export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [tenants, setTenants] = useState<string[]>([])
  const [sessions, setSessions] = useState<string[]>([])
  const [conversations, setConversations] = useState<string[]>([])
  const [active, setActive] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setOpen((o) => !o)
      }
      if (e.key === 'Escape') setOpen(false)
    }
    function onOpen() {
      setOpen(true)
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener('maskchain:command-open', onOpen)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('maskchain:command-open', onOpen)
    }
  }, [])

  useEffect(() => {
    if (!open) {
      setQuery('')
      setActive(0)
      return
    }
    inputRef.current?.focus()
    listTenants()
      .then((ts) => setTenants(ts.map((t) => t.slug)))
      .catch(() => {})
    fetch('/api/v1/sessions', { credentials: 'include' })
      .then((r) => (r.ok ? r.json() : null))
      .then((body) => {
        const d = body?.data ?? body
        const items = Array.isArray(d?.items) ? d.items : []
        setSessions(items.map((s: { session_id: string }) => s.session_id))
      })
      .catch(() => {})
    listConversations(1, 50)
      .then((res) => setConversations(res.items.map((c) => c.id)))
      .catch(() => {})
  }, [open])

  const results = useMemo<Result[]>(() => {
    const out: Result[] = []
    for (const n of NAV) {
      if (matches(query, n.label, n.keywords)) {
        out.push({ key: n.to, group: 'Navigate', label: n.label, icon: n.icon, to: n.to })
      }
    }
    if (query) {
      for (const t of tenants) {
        if (matches(query, t)) {
          out.push({ key: `t:${t}`, group: 'Tenants', label: t, hint: 'Open tenant', icon: Users, to: `/tenants/${t}` })
        }
      }
      for (const s of sessions) {
        if (matches(query, s)) {
          out.push({ key: `s:${s}`, group: 'Sessions', label: s.slice(0, 16) + '…', hint: 'Session ID', icon: Radio, to: '/sessions' })
        }
      }
      for (const c of conversations) {
        if (matches(query, c)) {
          out.push({ key: `c:${c}`, group: 'Conversations', label: c.slice(0, 16) + '…', hint: 'Conversation ID', icon: MessageSquare, to: '/conversations' })
        }
      }
    }
    return out.slice(0, 20)
  }, [query, tenants, sessions, conversations])

  useEffect(() => setActive(0), [query])

  const go = useCallback((to: string) => {
    setOpen(false)
    navigate(to)
  }, [navigate])

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive((a) => Math.min(a + 1, results.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive((a) => Math.max(a - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const r = results[active]
      if (r) go(r.to)
    }
  }

  if (!open) return null

  return (
    <div className="command-backdrop" onClick={() => setOpen(false)}>
      <div className="command-palette" role="dialog" aria-label="Command palette" onClick={(e) => e.stopPropagation()}>
        <div className="command-input-wrap">
          <Search size={16} className="text-muted" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Search pages, tenants, sessions, conversations…"
            aria-label="Search"
          />
          <kbd className="command-kbd">ESC</kbd>
        </div>
        <div className="command-results">
          {results.length === 0 && <div className="command-empty text-muted">No matches</div>}
          {results.map((r, i) => (
            <button
              key={r.key}
              type="button"
              className={`command-item${i === active ? ' active' : ''}`}
              onMouseEnter={() => setActive(i)}
              onClick={() => go(r.to)}
            >
              <r.icon size={15} className="command-icon" />
              <span className="command-label">{r.label}</span>
              {r.hint && <span className="command-hint">{r.hint}</span>}
              {i === active && <CornerDownLeft size={13} className="command-enter" />}
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
