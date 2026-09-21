import { useEffect, useMemo, useState } from 'react'
import { ShieldCheck, RefreshCw } from 'lucide-react'
import { Button, Card, EmptyState, PageHeader, Spinner, StatusPill, type StatusTone } from '../components/ui'
import { useToast } from '../components/Toast'
import { useWorkspace } from '../hooks/useWorkspace'
import { useShieldCatalog } from '../hooks/useShieldCatalog'
import {
  applyCompliancePack,
  getComplianceReport,
  listTenants,
  type ComplianceReport,
  type TenantListItem,
} from '../api/tenants'

const STATUS_TONE: Record<string, StatusTone> = {
  active: 'green',
  deviated: 'amber',
  missing: 'red',
}

// @sk-task ui-compliance-page: Standalone compliance pack console (apply + report)
export function Compliance() {
  const { toast } = useToast()
  const [workspace, setWorkspace] = useWorkspace()
  const [tenants, setTenants] = useState<TenantListItem[]>([])
  const [loadingTenants, setLoadingTenants] = useState(true)
  const [pack, setPack] = useState<string>('')
  const [report, setReport] = useState<ComplianceReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [reportError, setReportError] = useState(false)
  const { packs, loading: loadingPacks } = useShieldCatalog()

  const tenant = useMemo(() => tenants.find((t) => t.slug === workspace)?.slug ?? workspace, [tenants, workspace])

  useEffect(() => {
    if (packs.length === 0) {
      setPack('')
      return
    }
    if (!packs.some((p) => p.key === pack)) setPack(packs[0].key)
  }, [packs, pack])

  useEffect(() => {
    listTenants()
      .then((ts) => {
        setTenants(ts)
        if (!workspace && ts.length > 0) setWorkspace(ts[0].slug)
      })
      .catch(() => toast('Failed to load tenants', 'error'))
      .finally(() => setLoadingTenants(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const loadReport = async (slug: string, key: string) => {
    if (!slug) return
    setReportError(false)
    try {
      setReport(await getComplianceReport(slug, key))
    } catch {
      setReport(null)
      setReportError(true)
    }
  }

  useEffect(() => {
    if (tenant) loadReport(tenant, pack)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant, pack])

  async function handleApply() {
    if (!tenant) return
    setBusy(true)
    try {
      await applyCompliancePack(tenant, pack)
      toast(`Applied ${pack} to ${tenant}`, 'success')
      await loadReport(tenant, pack)
    } catch (e: any) {
      toast(e?.body?.error?.message ?? e?.message ?? 'Apply failed', 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div style={{ display: 'grid', gap: 16 }}>
      <PageHeader
        title="Compliance packs"
        subtitle="Apply a preset rule set to a tenant and audit drift from it."
        actions={
          <Button size="small" onClick={() => loadReport(tenant, pack)} disabled={!tenant}>
            <RefreshCw size={13} /> Re-check
          </Button>
        }
      />

      <Card>
        <div className="card-header-row">
          <h3>Target</h3>
        </div>
        <div className="filter-bar">
          <label className="filter-field">
            <span>Tenant</span>
            <select value={tenant} onChange={(e) => setWorkspace(e.target.value)} disabled={loadingTenants}>
              {tenants.length === 0 && <option value="">No tenants</option>}
              {tenants.map((t) => (
                <option key={t.slug} value={t.slug}>{`${t.name} (${t.slug})`}</option>
              ))}
            </select>
          </label>
          <label className="filter-field">
            <span>Pack</span>
            <select value={pack} onChange={(e) => setPack(e.target.value)} disabled={loadingPacks || packs.length === 0}>
              {packs.length === 0 && <option value="">No packs available</option>}
              {packs.map((p) => <option key={p.key} value={p.key}>{p.name}</option>)}
            </select>
          </label>
          <div className="u-grow" />
          <Button variant="primary" onClick={handleApply} disabled={busy || !tenant || !pack}>
            <ShieldCheck size={14} /> Apply {pack}
          </Button>
        </div>
      </Card>

      <Card>
        <div className="card-header-row">
          <h3>{pack} report {tenant ? `— ${tenant}` : ''}</h3>
          {report && (
            <StatusPill tone={report.rules.every((r) => r.status === 'active') ? 'green' : 'amber'} withDot={false}>
              {report.rules.filter((r) => r.status === 'active').length}/{report.rules.length} active
            </StatusPill>
          )}
        </div>
        {loadingTenants ? (
          <Spinner label="Loading..." />
        ) : reportError ? (
          <EmptyState message={`No report available for "${pack}". Apply the pack first, or check that the preset is loaded.`} />
        ) : !report || report.rules.length === 0 ? (
          <EmptyState message="No rules to report yet." />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Detector</th>
                  <th>Expected</th>
                  <th>Actual</th>
                  <th>Masking</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {report.rules.map((r) => (
                  <tr key={r.detector_type}>
                    <td><code>{r.detector_type}</code></td>
                    <td>{r.expected_reaction}</td>
                    <td>{r.actual_reaction ?? '—'}</td>
                    <td>{r.masking ? 'yes' : 'no'}</td>
                    <td>
                      <StatusPill tone={STATUS_TONE[r.status] ?? 'gray'}>{r.status}</StatusPill>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
