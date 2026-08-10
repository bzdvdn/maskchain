import { useState } from 'react'

export type SortDir = 'asc' | 'desc'

interface SortState<T> {
  key: keyof T | null
  dir: SortDir
}

export function useSort<T>(defaultKey: keyof T | null = null, defaultDir: SortDir = 'asc') {
  const [sort, setSort] = useState<SortState<T>>({ key: defaultKey, dir: defaultDir })

  function toggle(key: keyof T) {
    setSort((prev) => ({
      key,
      dir: prev.key === key && prev.dir === 'asc' ? 'desc' : 'asc',
    }))
  }

  return { key: sort.key, dir: sort.dir, toggle }
}

function valueOf<T>(item: T, key: keyof T): string | number | boolean {
  const v = item[key]
  if (typeof v === 'boolean') return v ? 1 : 0
  if (v == null) return ''
  return v as string | number
}

export function sortRows<T>(rows: T[], key: keyof T | null, dir: SortDir): T[] {
  if (!key) return rows
  const dirMul = dir === 'asc' ? 1 : -1
  return [...rows].sort((a, b) => {
    const av = valueOf(a, key)
    const bv = valueOf(b, key)
    if (av < bv) return -1 * dirMul
    if (av > bv) return 1 * dirMul
    return 0
  })
}
