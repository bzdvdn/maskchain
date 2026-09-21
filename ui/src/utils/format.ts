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

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

const RANGE_LABELS: Record<string, string> = {
  today: 'Today',
  yesterday: 'Yesterday',
  '7d': '7d',
  '30d': '30d',
  all: 'All',
}

function shortDay(d: Date): string {
  return `${MONTHS[d.getMonth()]} ${d.getDate()}`
}

// rangeLabel returns a short human label for a time-range selection. Presets map
// to their name; anything else (custom) renders as a date span, falling back to
// "Custom" when the bounds are missing or invalid.
export function rangeLabel(mode: string, from?: string, to?: string): string {
  if (RANGE_LABELS[mode]) return RANGE_LABELS[mode]
  const f = from ? new Date(from) : null
  const t = to ? new Date(to) : null
  if (f && t && !Number.isNaN(f.getTime()) && !Number.isNaN(t.getTime())) {
    return `${shortDay(f)} – ${shortDay(t)}`
  }
  return 'Custom'
}