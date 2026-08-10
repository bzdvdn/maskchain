interface Props {
  page: number
  totalPages: number
  total?: number
  onPage: (page: number) => void
}

export function Pagination({ page, totalPages, total, onPage }: Props) {
  return (
    <div className="pagination">
      <button type="button" className="btn btn-small" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        Prev
      </button>
      <span>
        Page {page} / {totalPages}
        {total !== undefined ? ` (${total})` : ''}
      </span>
      <button type="button" className="btn btn-small" disabled={page >= totalPages} onClick={() => onPage(page + 1)}>
        Next
      </button>
    </div>
  )
}
