import { useEffect, useMemo, useState } from 'react'
import { Play, Eraser } from 'lucide-react'
import { Button, Card, PageHeader, Spinner, StatusPill, type StatusTone } from '../components/ui'
import { useToast } from '../components/Toast'
import { useWorkspace } from '../hooks/useWorkspace'
import { listRoutes } from '../api/routing'
import { listTenants } from '../api/tenants'
import { sendPlayground } from '../api/playground'

interface RouteOption {
  model: string
  tenant: string
}

function shieldTone(status: string | null | undefined): StatusTone {
  if (status === 'blocked') return 'red'
  if (status === 'suspicious') return 'amber'
  if (status === 'clean') return 'green'
  return 'gray'
}

function extractContent(body: unknown): string | null {
  if (body && typeof body === 'object') {
    const choices = (body as { choices?: unknown }).choices
    if (Array.isArray(choices) && choices.length > 0) {
      const msg = (choices[0] as { message?: { content?: unknown } }).message
      if (msg && typeof msg.content === 'string') return msg.content
    }
  }
  return null
}

// @sk-task ui-playground: Test console that routes a prompt through the gateway.
export function Playground() {
  const { toast } = useToast()
  const [workspace] = useWorkspace()
  const [routes, setRoutes] = useState<RouteOption[]>([])
  const [loadingRoutes, setLoadingRoutes] = useState(true)
  const [apiKey, setApiKey] = useState('')
  const [model, setModel] = useState('')
  const [system, setSystem] = useState('')
  const [prompt, setPrompt] = useState('My email is alice@example.com — summarize this note.')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<{ status: number; shield?: string | null; body: unknown } | null>(null)

  useEffect(() => {
    Promise.all([listRoutes(), listTenants()])
      .then(([rs, ts]) => {
        const known = new Set(ts.map((t) => t.slug))
        const seen = new Set<string>()
        const options: RouteOption[] = []
        for (const r of rs) {
          const tenant = r.tenant || 'default'
          if (known.size > 0 && !known.has(tenant)) continue
          const key = `${tenant}|${r.model}`
          if (r.model && !seen.has(key)) {
            seen.add(key)
            options.push({ model: r.model, tenant })
          }
        }
        options.sort((a, b) => a.model.localeCompare(b.model))
        setRoutes(options)
        const preferred = workspace ? options.find((o) => o.tenant === workspace) : options[0]
        if (preferred) setModel(preferred.model)
      })
      .catch(() => toast('Failed to load models', 'error'))
      .finally(() => setLoadingRoutes(false))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!workspace) return
    const first = routes.find((o) => o.tenant === workspace)
    if (first) setModel(first.model)
  }, [workspace, routes])

  const models = useMemo(
    () => (workspace ? routes.filter((o) => o.tenant === workspace) : routes),
    [routes, workspace],
  )

  async function run() {
    if (!apiKey.trim()) { toast('Paste a tenant key to send the request', 'error'); return }
    if (!model) { toast('Select a model', 'error'); return }
    setRunning(true)
    setResult(null)
    try {
      const messages = system.trim()
        ? [{ role: 'system' as const, content: system }, { role: 'user' as const, content: prompt }]
        : [{ role: 'user' as const, content: prompt }]
      const res = await sendPlayground({ api_key: apiKey.trim(), model, messages })
      setResult({ status: res.status, shield: res.shield_status, body: res.body })
    } catch (e: any) {
      toast(e?.message ?? 'Request failed', 'error')
    } finally {
      setRunning(false)
    }
  }

  const content = result ? extractContent(result.body) : null

  return (
    <div style={{ display: 'grid', gap: 16 }}>
      <PageHeader
        title="Playground"
        subtitle="Send a test prompt through the shield, routing and budget pipeline."
        actions={
          <Button
            size="small"
            onClick={() => { setResult(null); setPrompt('') }}
            disabled={running}
          >
            <Eraser size={13} /> Clear
          </Button>
        }
      />

      <Card>
        <div className="card-header-row">
          <h3>Request</h3>
          {loadingRoutes && <Spinner label="Loading models..." />}
        </div>
        <div className="modal-body">
          <div className="form-field">
            <label>Tenant key</label>
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="sk-mc_… (raw key shown once at creation)"
              autoComplete="off"
            />
          </div>
          <div className="form-field">
            <label>Model</label>
            <select value={model} onChange={(e) => setModel(e.target.value)}>
              {models.length === 0 && <option value="">No routed models</option>}
              {models.map((m) => (
                <option key={`${m.tenant}|${m.model}`} value={m.model}>
                  {workspace ? m.model : `${m.model} (${m.tenant})`}
                </option>
              ))}
            </select>
          </div>
          <div className="form-field">
            <label>System prompt (optional)</label>
            <input value={system} onChange={(e) => setSystem(e.target.value)} placeholder="You are a helpful assistant." />
          </div>
          <div className="form-field">
            <label>User message</label>
            <textarea
              rows={4}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              style={{ width: '100%', resize: 'vertical' }}
            />
          </div>
        </div>
        <div className="form-actions">
          <Button variant="primary" onClick={run} disabled={running}>
            <Play size={14} /> {running ? 'Sending…' : 'Send'}
          </Button>
        </div>
      </Card>

      {result && (
        <Card>
          <div className="card-header-row">
            <h3>Response</h3>
            <span className="u-flex" style={{ gap: 8 }}>
              <StatusPill tone={result.status >= 200 && result.status < 300 ? 'green' : 'red'} withDot={false}>
                HTTP {result.status}
              </StatusPill>
              <StatusPill tone={shieldTone(result.shield)}>shield: {result.shield ?? 'n/a'}</StatusPill>
            </span>
          </div>
          {content !== null && (
            <div className="card" style={{ marginBottom: 12 }}>
              <h4 style={{ marginTop: 0 }}>Assistant</h4>
              <pre style={{ whiteSpace: 'pre-wrap', fontSize: 13 }}>{content}</pre>
            </div>
          )}
          <details>
            <summary className="text-muted">Raw gateway response</summary>
            <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{JSON.stringify(result.body, null, 2)}</pre>
          </details>
        </Card>
      )}
    </div>
  )
}
