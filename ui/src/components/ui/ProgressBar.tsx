type ProgressTone = 'auto' | 'accent' | 'warn' | 'danger'

interface Props {
  percentage: number
  tone?: ProgressTone
  ariaLabel?: string
}

function resolveTone(percentage: number): 'accent' | 'warn' | 'danger' {
  if (percentage >= 90) return 'danger'
  if (percentage >= 70) return 'warn'
  return 'accent'
}

export function ProgressBar({ percentage, tone = 'auto', ariaLabel }: Props) {
  const pct = Math.min(100, Math.max(0, percentage))
  const resolved = tone === 'auto' ? resolveTone(pct) : tone
  return (
    <div
      className="mc-progress"
      role="progressbar"
      aria-valuenow={Math.round(pct)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={ariaLabel}
    >
      <div className={`mc-progress-fill ${resolved}`} style={{ width: `${pct}%` }} />
    </div>
  )
}