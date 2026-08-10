interface Props {
  label?: string
}

export function Spinner({ label = 'Loading...' }: Props) {
  return (
    <div role="status" className="loading">
      <span className="spinner" aria-hidden="true" />
      {label}
    </div>
  )
}
