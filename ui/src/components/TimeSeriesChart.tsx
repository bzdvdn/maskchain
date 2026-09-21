import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { money } from '../utils/format'

export type ChartMetric = 'tokens' | 'cost' | 'requests'

interface Point {
  bucket: string
  input_tokens?: number
  output_tokens?: number
  cost?: number
  requests?: number
}

interface Props {
  data: Point[]
  height?: number
  compare?: Point[]
  metric?: ChartMetric
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

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

function fmtLabel(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffH = (now.getTime() - d.getTime()) / 3600_000
  if (diffH < 24) return `${pad2(d.getHours())}:${pad2(d.getMinutes())}`
  return `${d.getDate()}.${pad2(d.getMonth() + 1)} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

// metricValue formats a value for the tooltip; the unit follows the metric.
function metricValue(metric: ChartMetric, v: number): string {
  if (metric === 'cost') return money(v)
  if (metric === 'requests') return v.toLocaleString()
  return v.toLocaleString()
}

// axisTick formats a value compactly for the y-axis.
function axisTick(metric: ChartMetric, v: number): string {
  if (metric === 'cost') return v >= 1 ? `$${v.toFixed(1)}` : `$${v.toFixed(3)}`
  return fmtShort(v)
}

function ChartTooltip({ active, payload, label, metric }: {
  active?: boolean
  payload?: { name: string; value: number; color: string; dataKey: string }[]
  label?: string
  metric: ChartMetric
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
          <span style={{ color: 'var(--text)', fontWeight: 600 }}>{metricValue(metric, p.value)}</span>
        </div>
      ))}
    </div>
  )
}

const METRIC_LABEL: Record<ChartMetric, string> = {
  tokens: 'Tokens',
  cost: 'Cost',
  requests: 'Requests',
}

export function TimeSeriesChart({ data, height = 220, compare, metric = 'tokens' }: Props) {
  if (!data.length) {
    return (
      <div role="img" aria-label={`${METRIC_LABEL[metric]} trend: no data for this period`} className="text-muted" style={{ padding: 24, textAlign: 'center' }}>
        No data for this period
      </div>
    )
  }

  const allZero = data.every((d) => {
    if (metric === 'cost') return (d.cost ?? 0) === 0
    if (metric === 'requests') return (d.requests ?? 0) === 0
    return (d.input_tokens ?? 0) === 0 && (d.output_tokens ?? 0) === 0
  })
  if (allZero) {
    // A flat accent line at zero would read as activity; show a neutral state.
    return (
      <div role="img" aria-label={`${METRIC_LABEL[metric]} trend: no activity in this period`} className="text-muted" style={{ padding: 24, textAlign: 'center' }}>
        No activity in this period
      </div>
    )
  }

  const single = data.length === 1

  const chartData = data.map((d, i) => {
    const cmp = compare?.[i]
    const base = { bucket: d.bucket, label: fmtLabel(d.bucket) }
    if (metric === 'tokens') {
      return {
        ...base,
        Input: d.input_tokens ?? 0,
        Output: d.output_tokens ?? 0,
        Compare: cmp ? (cmp.input_tokens ?? 0) + (cmp.output_tokens ?? 0) : null,
      }
    }
    if (metric === 'cost') {
      return { ...base, Cost: d.cost ?? 0, Compare: cmp ? cmp.cost ?? 0 : null }
    }
    return { ...base, Requests: d.requests ?? 0, Compare: cmp ? cmp.requests ?? 0 : null }
  })

  return (
    <div role="img" aria-label={`${METRIC_LABEL[metric]} trend over ${data.length} buckets`} style={{ width: '100%' }}>
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
            tickFormatter={(v: number) => axisTick(metric, v)}
            tick={{ fill: TICK, fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            width={52}
          />
          <Tooltip content={<ChartTooltip metric={metric} />} cursor={{ stroke: 'var(--text-muted)', strokeDasharray: '3 3', strokeOpacity: 0.4 }} />

          {metric === 'tokens' ? (
            <>
              <Area
                type="monotone"
                dataKey="Input"
                stroke={INPUT}
                strokeWidth={2}
                fill="url(#gradInput)"
                dot={single ? { r: 3 } : false}
                activeDot={{ r: 4, strokeWidth: 0, fill: INPUT }}
              />
              <Area
                type="monotone"
                dataKey="Output"
                stroke={OUTPUT}
                strokeWidth={2}
                fill="url(#gradOutput)"
                dot={single ? { r: 3 } : false}
                activeDot={{ r: 4, strokeWidth: 0, fill: OUTPUT }}
              />
            </>
          ) : (
            <Area
              type="monotone"
              dataKey={metric === 'cost' ? 'Cost' : 'Requests'}
              stroke={INPUT}
              strokeWidth={2}
              fill="url(#gradInput)"
              dot={single ? { r: 3 } : false}
              activeDot={{ r: 4, strokeWidth: 0, fill: INPUT }}
            />
          )}

          {compare && (
            <Area
              type="monotone"
              dataKey="Compare"
              stroke={TICK}
              strokeWidth={1.5}
              strokeDasharray="4 4"
              fill="none"
              dot={false}
              activeDot={false}
            />
          )}
        </AreaChart>
      </ResponsiveContainer>

      <div style={{ display: 'flex', gap: 20, justifyContent: 'center', marginTop: 10 }}>
        {metric === 'tokens' ? (
          <>
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: TICK }}>
              <span style={{ width: 10, height: 10, borderRadius: 2, background: INPUT, display: 'inline-block', flexShrink: 0 }} /> Input
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: TICK }}>
              <span style={{ width: 10, height: 10, borderRadius: 2, background: OUTPUT, display: 'inline-block', flexShrink: 0 }} /> Output
            </span>
          </>
        ) : (
          <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: TICK }}>
            <span style={{ width: 10, height: 10, borderRadius: 2, background: INPUT, display: 'inline-block', flexShrink: 0 }} />
            {metric === 'cost' ? 'Cost' : 'Requests'}
          </span>
        )}
        {compare && (
          <span style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11, color: TICK }}>
            <span style={{ width: 14, height: 0, borderTop: '1.5px dashed var(--text-muted)', display: 'inline-block' }} /> prev period
          </span>
        )}
      </div>
      {single && (
        <div className="text-muted" style={{ fontSize: 11, textAlign: 'center', marginTop: 4 }}>
          Single data point — widen the range for a trend.
        </div>
      )}
    </div>
  )
}
