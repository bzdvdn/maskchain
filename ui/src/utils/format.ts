function pad(n: number): string {
  return String(n).padStart(2, '0')
}

export function absDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function relativeTime(iso: string, now: Date = new Date()): string {
  const t = new Date(iso)
  if (Number.isNaN(t.getTime())) return '—'
  const diffMs = t.getTime() - now.getTime()
  const minutes = Math.floor(Math.abs(diffMs) / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return diffMs > 0 ? `in ${minutes}m` : `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return diffMs > 0 ? `in ${hours}h` : `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return diffMs > 0 ? `in ${days}d` : `${days}d ago`
  return absDate(iso)
}

export function groupNum(n: number | string): string {
  const num = typeof n === 'string' ? Number(n) : n
  if (!Number.isFinite(num)) return '—'
  return String(Math.trunc(num)).replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}

export function money(value: number, currency = 'USD'): string {
  if (!Number.isFinite(value)) return '—'
  const symbol = currency === 'USD' ? '$' : currency ? `${currency} ` : ''
  const fixed = value.toFixed(2)
  const [int, dec] = fixed.split('.')
  return `${symbol}${groupNum(int)}.${dec}`
}

export function fmtTokens(n: number): string {
  if (!Number.isFinite(n)) return '—'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}