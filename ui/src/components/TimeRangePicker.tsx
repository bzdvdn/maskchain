import { useState, useMemo, useCallback } from 'react'

interface Preset {
  label: string
  value: string
  range: () => { from: Date; to: Date }
}

const presets: Preset[] = [
  {
    label: 'Today', value: 'today',
    range: () => {
      const now = new Date()
      const from = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      return { from, to: now }
    },
  },
  {
    label: 'Yesterday', value: 'yesterday',
    range: () => {
      const now = new Date()
      const from = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
      const to = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      return { from, to }
    },
  },
  {
    label: '7d', value: '7d',
    range: () => {
      const to = new Date()
      const from = new Date(to.getTime() - 7 * 86400_000)
      return { from, to }
    },
  },
  {
    label: '30d', value: '30d',
    range: () => {
      const to = new Date()
      const from = new Date(to.getTime() - 30 * 86400_000)
      return { from, to }
    },
  },
  {
    label: 'All', value: 'all',
    range: () => {
      const to = new Date()
      const from = new Date('2024-01-01T00:00:00Z')
      return { from, to }
    },
  },
]

export type RangeValue = { mode: string; from: string; to: string }

function fmtLocal(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const h = String(d.getHours()).padStart(2, '0')
  const min = String(d.getMinutes()).padStart(2, '0')
  return `${y}-${m}-${day}T${h}:${min}`
}

function parseLocal(v: string): Date {
  const d = new Date(v)
  return isNaN(d.getTime()) ? new Date() : d
}

interface Props {
  value: RangeValue
  onChange: (v: RangeValue) => void
}

export function TimeRangePicker({ value, onChange }: Props) {
  const isCustom = value.mode === 'custom'
  const [customFrom, setCustomFrom] = useState(() => fmtLocal(new Date(Date.now() - 7 * 86400_000)))
  const [customTo, setCustomTo] = useState(() => fmtLocal(new Date()))

  const handlePreset = useCallback((preset: Preset) => {
    const { from, to } = preset.range()
    onChange({ mode: preset.value, from: from.toISOString(), to: to.toISOString() })
  }, [onChange])

  const enterCustom = useCallback(() => {
    const now = new Date()
    const then = new Date(now.getTime() - 7 * 86400_000)
    const f = fmtLocal(then)
    const t = fmtLocal(now)
    setCustomFrom(f)
    setCustomTo(t)
    onChange({ mode: 'custom', from: parseLocal(f).toISOString(), to: parseLocal(t).toISOString() })
  }, [onChange])

  const applyCustom = useCallback(() => {
    const from = parseLocal(customFrom)
    const to = parseLocal(customTo)
    onChange({ mode: 'custom', from: from.toISOString(), to: to.toISOString() })
  }, [customFrom, customTo, onChange])

  return (
    <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexWrap: 'wrap' }}>
      {presets.map((p) => (
        <button
          key={p.value}
          className="btn btn-small"
          onClick={() => handlePreset(p)}
          style={value.mode === p.value ? { borderColor: 'var(--accent)', color: 'var(--accent)' } : {}}
        >
          {p.label}
        </button>
      ))}
      <button
        className="btn btn-small"
        onClick={isCustom ? applyCustom : enterCustom}
        style={isCustom ? { borderColor: 'var(--accent)', color: 'var(--accent)' } : {}}
      >
        {isCustom ? 'Apply' : 'Custom'}
      </button>
      {isCustom && (
        <>
          <input
            type="datetime-local"
            value={customFrom}
            onChange={(e) => setCustomFrom(e.target.value)}
            style={{
              padding: '4px 8px', background: 'var(--bg)', border: '1px solid var(--border)',
              borderRadius: 4, color: 'var(--text)', fontSize: 12, fontFamily: 'inherit',
            }}
          />
          <span style={{ color: 'var(--text-muted)', fontSize: 12 }}>—</span>
          <input
            type="datetime-local"
            value={customTo}
            onChange={(e) => setCustomTo(e.target.value)}
            style={{
              padding: '4px 8px', background: 'var(--bg)', border: '1px solid var(--border)',
              borderRadius: 4, color: 'var(--text)', fontSize: 12, fontFamily: 'inherit',
            }}
          />
        </>
      )}
    </div>
  )
}

export function useRange(v: RangeValue): { from: string; to: string } {
  return useMemo(() => ({ from: v.from, to: v.to }), [v.from, v.to])
}

export const defaultRange: RangeValue = { mode: '7d', from: '', to: '' }
