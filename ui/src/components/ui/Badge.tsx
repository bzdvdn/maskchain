interface Props {
  value: string
}

const tone = (v: string) => {
  if (v === 'ok' || v === 'active' || v === 'healthy' || v === 'up' || v === 'yes') return 'badge-up'
  if (v === 'error' || v === 'blocked' || v === 'expired' || v === 'down' || v === 'no') return 'badge-down'
  return 'badge-warn'
}

export function Badge({ value }: Props) {
  return <span className={`badge ${tone(value)}`}>{value}</span>
}
