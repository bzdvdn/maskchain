export type StatusTone = 'green' | 'amber' | 'red' | 'blue' | 'gray'

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