import type { ButtonHTMLAttributes } from 'react'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'
import type { SortDir } from '../../hooks/useSort'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  active: boolean
  dir: SortDir
}

export function SortHeader({ active, dir, children, ...rest }: Props) {
  return (
    <button
      type="button"
      className="sort-header"
      aria-sort={active ? (dir === 'asc' ? 'ascending' : 'descending') : 'none'}
      {...rest}
    >
      {children}
      {active ? (
        dir === 'asc' ? <ArrowUp size={12} /> : <ArrowDown size={12} />
      ) : (
        <ChevronsUpDown size={12} />
      )}
    </button>
  )
}
