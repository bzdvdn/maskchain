import type { HTMLAttributes } from 'react'

export function Card({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={`card${className ? ` ${className}` : ''}`} {...rest} />
}

export function CardHeader({ title, actions }: { title: string; actions?: React.ReactNode }) {
  return (
    <div className="card-header-row">
      <h3>{title}</h3>
      {actions}
    </div>
  )
}
