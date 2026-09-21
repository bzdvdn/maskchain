import { StatusPill, type StatusTone } from './Status'

interface Props {
  value: string
}

// Badge is a thin compatibility wrapper over StatusPill so there is a single
// status rendering implementation across the console.
function tone(v: string): StatusTone {
  const val = v.toLowerCase()
  if (['ok', 'active', 'healthy', 'up', 'yes', 'enabled', 'on'].includes(val)) return 'green'
  if (['error', 'blocked', 'expired', 'down', 'no', 'disabled', 'revoked'].includes(val)) return 'red'
  return 'amber'
}

export function Badge({ value }: Props) {
  return (
    <StatusPill tone={tone(value)} withDot={false}>
      {value}
    </StatusPill>
  )
}
