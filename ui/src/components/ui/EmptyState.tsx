import type { ReactNode } from 'react'

interface Props {
  message: string
  action?: ReactNode
}

export function EmptyState({ message, action }: Props) {
  return (
    <div className="empty-state">
      <p>{message}</p>
      {action && <div style={{ marginTop: 4 }}>{action}</div>}
    </div>
  )
}