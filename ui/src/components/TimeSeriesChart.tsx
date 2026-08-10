import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

interface Point {
  bucket: string
  input_tokens: number
  output_tokens: number
}

interface Props {
  data: Point[]
  height?: number
}

const INPUT = 'var(--accent)'
const OUTPUT = 'var(--green)'
const TICK = 'var(--text-muted)'
const GRID = 'var(--border)'

function fmtShort(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return String(n)
}

function fmtTick(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffH = (now.getTime() - d.getTime()) / 3600_000
  if (diffH < 24) return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  if (diffH < 168) return `${d.getDate()}.${d.getMonth() + 1} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  return `${d.getDate()}.${d.getMonth() + 1}`
}

function fmtLabel(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffH = (now.getTime() - d.getTime()) / 3600_000
  if (diffH < 24) return d.toLocaleString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
}

function ChartTooltip({ active, payload, label }: {
  active?: boolean
  payload?: { name: string; value: number; color: string; dataKey: string }[]
  label?: string
}) {
  if (!active || !payload?.length) return null
  return (
    <div
      style={{
        background: 'var(--surface)',
        border: '1px solid var(--border)',
        borderRadius: 8,
        padding: '8px 12px',
        fontSize: 12,
        boxShadow: 'var(--shadow)',
      }}
    >
      <div style={{ color: TICK, marginBottom: 6 }}>{label ? fmtLabel(label) : ''}</div>
      {payload.map((p) => (
        <div key={p.dataKey} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ width: 8, height: 8, borderRadius: 2, background: p.color, display: 'inline-block', flexShrink: 0 }} />
          <span style={{ color: TICK, textTransform: 'capitalize' }}>{p.name}:</span>
          <span style={{ color: 'var(--text)', fontWeight: 600 }}>{p.value.toLocaleString()}</span>
        </div>
      ))}
    </div>
  )
}

export function TimeSeriesChart({ data, height = 220 }: Props) {
  if (!data.length) {
    return (
      <div className="text-muted" style={{ padding: 24, textAlign: 'center' }}>
        No data for this period
      </div>
    )
  }

  const chartData = data.map((d) => ({
    ...d,
    label: fmtLabel(d.bucket),
    Input: d.input_tokens,
    Output: d.output_tokens,
  }))

  return (
    <div style={{ width: '100%' }}>
      <ResponsiveContainer width="100%" height={height}>
        <AreaChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id="gradInput" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={INPUT} stopOpacity={0.45} />
              <stop offset="100%" stopColor={INPUT} stopOpacity={0.02} />
            </linearGradient>
            <linearGradient id="gradOutput" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={OUTPUT} stopOpacity={0.4} />
              <stop offset="100%" stopColor={OUTPUT} stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={GRID} strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="bucket"
            tickFormatter={fmtTick}
            tick={{ fill: TICK, fontSize: 10 }}
            tickLine={false}
            axisLine={{ stroke: GRID }}
            minTickGap={24}
            interval="preserveStartEnd"
          />
          <YAxis
            tickFormatter={(v: number) => fmtShort(v)}
            tick={{ fill: TICK, fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            width={44}
          />
          <Tooltip content={<ChartTooltip />} cursor={{ stroke: 'var(--text-muted)', strokeDasharray: '3 3', strokeOpacity: 0.4 }} />
          <Area
            type="monotone"
            dataKey="Input"
            stroke={INPUT}
            strokeWidth={2}
            fill="url(#gradInput)"
            dot={false}
            activeDot={{ r: 4, strokeWidth: 0, fill: INPUT }}
          />
          <Area
            type="monotone"
            dataKey="Output"
            stroke={OUTPUT}
            strokeWidth={2}
            fill="url(#gradOutput)"
            dot={false}
            activeDot={{ r: 4, strokeWidth: 0, fill: OUTPUT }}
          />
        </AreaChart>
      </ResponsiveContainer>

      <div style={{ display: 'flex', gap: 20, justifyContent: 'center', marginTop: 4 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: TICK }}>
          <span style={{ width: 10, height: 10, borderRadius: 2, background: INPUT, display: 'inline-block', flexShrink: 0 }} /> Input
        </span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: TICK }}>
          <span style={{ width: 10, height: 10, borderRadius: 2, background: OUTPUT, display: 'inline-block', flexShrink: 0 }} /> Output
        </span>
      </div>
    </div>
  )
}
