import { useState, useRef, useEffect } from 'react'

interface Point {
  bucket: string
  input_tokens: number
  output_tokens: number
}

interface Props {
  data: Point[]
  height?: number
}

const INPUT = '#6c8aff'
const OUTPUT = '#34d399'
const TICK = '#8b8fa8'
const GRID = 'rgba(255,255,255,0.06)'
const HOVER = 'rgba(255,255,255,0.08)'

export function TimeSeriesChart({ data, height = 195 }: Props) {
  const [hover, setHover] = useState<number | null>(null)
  const ref = useRef<HTMLDivElement>(null)
  const [cw, setCw] = useState(0)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    setCw(el.clientWidth)
    const ro = new ResizeObserver((es) => {
      for (const e of es) setCw(e.contentRect.width)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  if (!data.length) return <div className="text-muted" style={{ padding: 24, textAlign: 'center' }}>No data for this period</div>

  const plotY = 10
  const plotH = 150
  const vbW = cw || 800

  const count = data.length
  const totals = data.map((d) => d.input_tokens + d.output_tokens)
  const maxV = Math.max(...totals, 1)

  const padL = 48
  const padR = 12
  const plotW = vbW - padL - padR

  const stepX = plotW / count
  const barW = Math.min(stepX * 0.7, 28)
  const gap = stepX - barW

  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => ({
    y: plotY + plotH * (1 - f),
    label: f === 0 ? '0' : f === 1 ? fmtShort(maxV) : fmtShort(Math.round(maxV * f)),
  }))

  const labelEvery = count > 12 ? Math.ceil(count / 10) : 1
  const baseline = plotY + plotH
  const labelY = height - 4

  return (
    <div ref={ref} style={{ width: '100%' }}>
      <svg width={vbW} height={height} style={{ maxWidth: '100%', display: 'block', overflow: 'visible' }}>
        {yTicks.map((gl, i) => (
          <g key={i}>
            <line x1={padL} y1={gl.y} x2={vbW - padR} y2={gl.y} stroke={GRID} strokeWidth={1} />
            <text x={padL - 6} y={gl.y + 3} fill={TICK} fontSize="9" textAnchor="end">{gl.label}</text>
          </g>
        ))}

        <line x1={padL} y1={baseline} x2={vbW - padR} y2={baseline} stroke={GRID} strokeWidth={1} />

        {data.map((d, i) => {
          const inH = (d.input_tokens / maxV) * plotH
          const outH = (d.output_tokens / maxV) * plotH
          const bx = padL + i * stepX + gap / 2
          const inY = baseline - inH
          const outY = inY - outH
          const cx = bx + barW / 2
          const isHover = hover === i

          return (
            <g key={i} onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)} style={{ cursor: 'pointer' }}>
              {isHover && (
                <rect x={padL + i * stepX} y={plotY} width={stepX} height={plotH} fill={HOVER} rx="2" />
              )}
              {outH > 0 && (
                <rect x={bx} y={outY} width={barW} height={outH} fill={OUTPUT} rx="1.5" opacity={isHover ? 1 : 0.85} />
              )}
              {inH > 0 && (
                <rect x={bx} y={inY} width={barW} height={inH} fill={INPUT} rx="1.5" opacity={isHover ? 1 : 0.85} />
              )}

              {(i % labelEvery === 0 || isHover) && (
                <text
                  x={cx}
                  y={labelY}
                  textAnchor="end"
                  fill={isHover ? '#fff' : TICK}
                  fontSize="8"
                  fontWeight={isHover ? '600' : '400'}
                  transform={`rotate(-30, ${cx}, ${labelY})`}
                >
                  {fmtTick(d.bucket)}
                </text>
              )}

              {isHover && (
                <>
                  <line x1={cx} y1={plotY} x2={cx} y2={baseline} stroke={TICK} strokeWidth={1} strokeDasharray="3,3" opacity={0.4} />
                  <rect x={Math.min(cx + 8, vbW - 160)} y={Math.max(outY - 46, 2)} width={150} height={44} rx="4" fill="#1a1a2e" stroke="rgba(255,255,255,0.1)" strokeWidth={1} />
                  <text x={Math.min(cx + 14, vbW - 154)} y={Math.max(outY - 30, 8)} fill={TICK} fontSize="10">
                    {fmtLabel(d.bucket)}
                  </text>
                  <text x={Math.min(cx + 14, vbW - 154)} y={Math.max(outY - 14, 24)} fill={INPUT} fontSize="11" fontWeight="600">
                    ▲ {d.input_tokens.toLocaleString()}
                  </text>
                  <text x={Math.min(cx + 76, vbW - 80)} y={Math.max(outY - 14, 24)} fill={OUTPUT} fontSize="11" fontWeight="600">
                    ▼ {d.output_tokens.toLocaleString()}
                  </text>
                </>
              )}
            </g>
          )
        })}
      </svg>

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

function fmtLabel(iso: string): string {
  const d = new Date(iso)
  const now = new Date()
  const diffH = (now.getTime() - d.getTime()) / 3600_000
  if (diffH < 24) return d.toLocaleString('ru-RU', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleString('ru-RU', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })
}

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
