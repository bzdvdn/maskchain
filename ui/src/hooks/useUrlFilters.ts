import { useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'

// @sk-task ui-production-readiness#T4.1: URL-persisted filters (AC-009)
//
// useUrlFilters keeps a page's filters in the query string so a filtered view is
// shareable and survives navigation. Empty values are removed from the URL.
export interface UrlFilters {
  get: (key: string, fallback?: string) => string
  getInt: (key: string, fallback: number) => number
  set: (patch: Record<string, string | number | undefined>) => void
}

export function useUrlFilters(): UrlFilters {
  const [params, setParams] = useSearchParams()

  const get = useCallback(
    (key: string, fallback = '') => params.get(key) ?? fallback,
    [params],
  )

  const getInt = useCallback(
    (key: string, fallback: number) => {
      const raw = params.get(key)
      if (raw === null) return fallback
      const n = Number.parseInt(raw, 10)
      return Number.isFinite(n) ? n : fallback
    },
    [params],
  )

  const set = useCallback(
    (patch: Record<string, string | number | undefined>) => {
      const next = new URLSearchParams(params)
      for (const [key, value] of Object.entries(patch)) {
        if (value === undefined || value === '' || value === null) next.delete(key)
        else next.set(key, String(value))
      }
      setParams(next, { replace: true })
    },
    [params, setParams],
  )

  return { get, getInt, set }
}
