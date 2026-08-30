import type { ReactNode } from 'react'

interface Props {
  title?: string
  message: string
  action?: ReactNode
}

export function EmptyState({ title, message, action }: Props) {
  return (
    <div className="empty-state">
      {title && <h4 style={{ marginBottom: 6 }}>{title}</h4>}
      <p>{message}</p>
      {action && <div style={{ marginTop: 12 }}>{action}</div>}
    </div>
  )
}