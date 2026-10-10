import { useCallback, useEffect, useState } from 'react'
import type { StatusTone } from '../components/ui'
import { listBudgets, listBudgetsByTenant } from '../api/budgets'
import { listProviders } from '../api/routing'
import { listKeys } from '../api/keys'
import { listConversations } from '../api/conversations'
import { applyCompliancePack, getComplianceReport, listTenants } from '../api/tenants'
import { getShieldCatalog } from '../api/shield'
import { money } from '../utils/format'

export interface AttentionItem {
  key: string
  tone: StatusTone
  critical: boolean
  title: string
  detail: string
  to?: string
  actionLabel?: string
  action?: () => Promise<void>
}

export interface SnoozeOption {
  label: string
  ms: number
}

export const SNOOZE_OPTIONS: SnoozeOption[] = [
  { label: '1h', ms: 60 * 60 * 1000 },
  { label: '24h', ms: 24 * 60 * 60 * 1000 },
  { label: '7d', ms: 7 * 24 * 60 * 60 * 1000 },
]

const ACK_KEY = 'maskchain.attention.acked'
const SNOOZE_KEY = 'maskchain.attention.snoozed'

function readJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return fallback
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

export function isSnoozed(snoozed: Record<string, number>, key: string, now: number): boolean {
  const until = snoozed[key]
  return typeof until === 'number' && Number.isFinite(until) && until > now
}

// @sk-task operator-console-v2-1#T1.1: build the attention items (AC-001, AC-002, AC-003, AC-004)
//
// computeAttention collects the dashboard signals. The compliance source is
// bounded to the active workspace tenant (skipped without a workspace) so it
// never fans out over every tenant.
export async function computeAttention(workspace: string): Promise<AttentionItem[]> {
  const items: AttentionItem[] = []

  try {
    const budgets = workspace ? await listBudgetsByTenant(workspace) : await listBudgets()
    const list = budgets?.data ?? []
    for (const b of list) {
      if (!b.hard_limit) continue
      const pct = (b.spent / b.hard_limit) * 100
      if (pct >= 100) {
        items.push({
          key: `budget:${b.id}`,
          tone: 'red',
          critical: true,
          title: 'Budget exhausted',
          detail: `${b.tenant_id} hit ${money(b.hard_limit)} / ${b.type}`,
          to: '/budgets',
        })
      } else if (pct >= 70) {
        items.push({
          key: `budget:${b.id}`,
          tone: 'amber',
          critical: false,
          title: `Budget at ${Math.round(pct)}%`,
          detail: `${b.tenant_id} — ${money(b.spent)} of ${money(b.hard_limit)}`,
          to: '/budgets',
        })
      }
    }
  } catch {
    /* attention degrades gracefully */
  }

  try {
    const providers = await listProviders()
    for (const p of Array.isArray(providers) ? providers : []) {
      if (p.status === 'down') {
        items.push({ key: `provider:${p.name}`, tone: 'red', critical: true, title: 'Provider down', detail: `${p.name} unreachable`, to: '/providers' })
      } else if (p.status === 'degraded') {
        items.push({ key: `provider:${p.name}`, tone: 'amber', critical: false, title: 'Provider degraded', detail: p.name, to: '/providers' })
      }
    }
  } catch {
    /* ignore */
  }

  try {
    const keys = await listKeys()
    const rows = Array.isArray(keys?.data) ? keys.data : []
    const expiring = rows.filter((k) => {
      if (!k.expires_at) return false
      const t = new Date(k.expires_at).getTime()
      return t > Date.now() && t - Date.now() < 7 * 86400_000
    })
    if (expiring.length > 0) {
      items.push({
        key: 'keys:expiring',
        tone: 'blue',
        critical: false,
        title: 'Keys expiring',
        detail: `${expiring.length} virtual key${expiring.length > 1 ? 's' : ''} expire within 7 days`,
        to: '/keys',
      })
    }
  } catch {
    /* ignore */
  }

  try {
    const conv = await listConversations(1, 100)
    const blocked = (Array.isArray(conv.items) ? conv.items : []).filter((c) => c.status === 'blocked').length
    if (blocked > 0) {
      items.push({
        key: 'blocked:last100',
        tone: 'amber',
        critical: false,
        title: 'Requests blocked',
        detail: `${blocked} messages blocked · last 100`,
        to: '/conversations',
      })
    }
  } catch {
    /* ignore */
  }

  // @sk-task operator-console-v2-1#T1.1: compliance deviation is bounded to the workspace (DEC-003)
  if (workspace) {
    try {
      const tenants = await listTenants()
      if (Array.isArray(tenants) && tenants.some((t) => t.slug === workspace)) {
        const catalog = await getShieldCatalog()
        for (const pack of catalog.packs ?? []) {
          try {
            const report = await getComplianceReport(workspace, pack.key)
            const deviated = report.rules.filter((r) => r.status === 'deviated').length
            if (deviated > 0) {
              items.push({
                key: `compliance:${workspace}:${pack.key}`,
                tone: 'amber',
                critical: false,
                title: 'Compliance deviation',
                detail: `${workspace} · pack ${pack.key} has ${deviated} rule${deviated > 1 ? 's' : ''} out of sync`,
                to: `/tenants/${workspace}?tab=compliance`,
                actionLabel: `Re-apply ${pack.key}`,
                action: () => applyCompliancePack(workspace, pack.key).then(() => undefined),
              })
            }
          } catch {
            /* per-pack failure is non-fatal */
          }
        }
      }
    } catch {
      /* ignore */
    }
  }

  // Critical first, then warnings, keeping insertion order within a tone.
  const rank = (i: AttentionItem) => (i.critical ? 0 : i.tone === 'amber' ? 1 : 2)
  return items.sort((a, b) => rank(a) - rank(b))
}

export interface AttentionState {
  items: AttentionItem[]
  criticalCount: number
  reload: () => Promise<void>
  ack: (key: string) => void
  snooze: (key: string, ms: number) => void
}

// @sk-task operator-console-v2-1#T1.1: shared attention store with ack/snooze (DEC-002)
export function useAttention(workspace: string): AttentionState {
  const [all, setAll] = useState<AttentionItem[]>([])
  const [acked, setAcked] = useState<string[]>(() => readJSON<string[]>(ACK_KEY, []))
  const [snoozed, setSnoozed] = useState<Record<string, number>>(() => readJSON<Record<string, number>>(SNOOZE_KEY, {}))

  const reload = useCallback(async () => {
    setAll(await computeAttention(workspace))
  }, [workspace])

  useEffect(() => {
    void reload()
  }, [reload])

  const ack = useCallback((key: string) => {
    setAcked((prev) => {
      if (prev.includes(key)) return prev
      const next = [...prev, key]
      try {
        localStorage.setItem(ACK_KEY, JSON.stringify(next))
      } catch {
        /* ignore quota errors */
      }
      return next
    })
  }, [])

  const snooze = useCallback((key: string, ms: number) => {
    setSnoozed((prev) => {
      const next = { ...prev, [key]: Date.now() + ms }
      try {
        localStorage.setItem(SNOOZE_KEY, JSON.stringify(next))
      } catch {
        /* ignore quota errors */
      }
      return next
    })
  }, [])

  const now = Date.now()
  const visible = all.filter((i) => !acked.includes(i.key) && !isSnoozed(snoozed, i.key, now))
  const criticalCount = visible.filter((i) => i.critical).length

  return { items: visible, criticalCount, reload, ack, snooze }
}
