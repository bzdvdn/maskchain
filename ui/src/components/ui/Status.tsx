export type StatusTone = 'green' | 'amber' | 'red' | 'blue' | 'gray'

// @sk-task ui-production-readiness#T2.3: Single status tone mapping (AC-007)
export function statusTone(value: string): StatusTone {
  const val = value.toLowerCase()
  if (['ok', 'active', 'healthy', 'up', 'yes', 'enabled', 'on'].includes(val)) return 'green'
  if (['error', 'blocked', 'expired', 'down', 'no', 'disabled', 'revoked'].includes(val)) return 'red'
  return 'amber'
}

interface DotProps {
  tone?: StatusTone
  pulse?: boolean
  title?: string
}

export function StatusDot({ tone = 'gray', pulse = false, title }: DotProps) {
  return <span className={`mc-dot ${tone}${pulse ? ' pulse' : ''}`} aria-hidden="true" title={title} />
}

interface PillProps {
  tone?: StatusTone
  children: React.ReactNode
  withDot?: boolean
}

export function StatusPill({ tone = 'gray', children, withDot = true }: PillProps) {
  return (
    <span className={`mc-pill ${tone}`}>
      {withDot && <StatusDot tone={tone} />}
      {children}
    </span>
  )
}