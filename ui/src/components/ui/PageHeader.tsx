interface Props {
  title: string
  subtitle?: string
  actions?: React.ReactNode
}

export function PageHeader({ title, subtitle, actions }: Props) {
  return (
    <div className="mc-page-header">
      <div>
        <h1 className="mc-page-title">{title}</h1>
        {subtitle && <div className="mc-page-sub">{subtitle}</div>}
      </div>
      {actions && <div className="mc-page-actions">{actions}</div>}
    </div>
  )
}