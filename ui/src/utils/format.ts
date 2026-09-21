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

const HOUR_MS = 3600_000
const DAY_MS = 86_400_000

function clock(d: Date): string {
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function dayMonth(d: Date): string {
  return `${d.getDate()}.${pad(d.getMonth() + 1)}`
}

// chartTickLabel formats a chart axis tick from the requested range span:
// clock time for short ranges, date plus time up to a week, and date only
// beyond that (so daily buckets never show a fake 00:00).
export function chartTickLabel(iso: string, spanMs: number): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  if (spanMs <= 48 * HOUR_MS) return clock(d)
  if (spanMs <= 7 * DAY_MS) return `${dayMonth(d)} ${clock(d)}`
  return dayMonth(d)
}

// chartTooltipLabel always shows the full date and time of a bucket.
export function chartTooltipLabel(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return `${dayMonth(d)}.${d.getFullYear()} ${clock(d)}`
}

const RANGE_SPAN_MS: Record<string, number> = {
  today: DAY_MS,
  yesterday: 2 * DAY_MS,
  '7d': 7 * DAY_MS,
  '30d': 30 * DAY_MS,
}

// rangeSpanMs returns the span of a range selection in milliseconds so charts
// can pick the right label format. Presets map to their window, "all" is
// unbounded, and a custom range uses its bounds.
export function rangeSpanMs(mode: string, from?: string, to?: string): number {
  if (mode === 'all') return Number.POSITIVE_INFINITY
  const preset = RANGE_SPAN_MS[mode]
  if (preset) return preset
  const f = from ? new Date(from).getTime() : NaN
  const t = to ? new Date(to).getTime() : NaN
  if (Number.isFinite(f) && Number.isFinite(t) && t >= f) return t - f
  return 0
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