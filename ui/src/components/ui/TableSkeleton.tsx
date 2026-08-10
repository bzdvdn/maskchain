interface Props {
  rows?: number
  cols?: number
}

export function TableSkeleton({ rows = 5, cols = 7 }: Props) {
  return (
    <div className="skeleton-table" role="status" aria-label="Loading table data">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skeleton-row">
          {Array.from({ length: cols }).map((_, j) => (
            <div key={j} className="skeleton" style={{ width: j === 0 ? '60%' : '85%' }} />
          ))}
        </div>
      ))}
    </div>
  )
}
