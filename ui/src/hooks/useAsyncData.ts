import { useCallback, useEffect, useRef, useState } from 'react'

interface Options<T> {
  enabled?: boolean
  initial?: T
  onError?: (err: unknown) => void
}

interface Result<T> {
  data: T | undefined
  loading: boolean
  error: unknown
  refetch: () => void
}

export function useAsyncData<T>(
  fn: () => Promise<T>,
  deps: unknown[],
  { enabled = true, initial, onError }: Options<T> = {},
): Result<T> {
  const [data, setData] = useState<T | undefined>(initial)
  const [loading, setLoading] = useState<boolean>(enabled)
  const [error, setError] = useState<unknown>(null)
  const fnRef = useRef(fn)
  fnRef.current = fn
  const onErrorRef = useRef(onError)
  onErrorRef.current = onError
  const [tick, setTick] = useState(0)

  const refetch = useCallback(() => setTick((t) => t + 1), [])

  useEffect(() => {
    if (!enabled) {
      setLoading(false)
      return
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    fnRef.current()
      .then((res) => {
        if (!cancelled) setData(res)
      })
      .catch((err) => {
        if (cancelled) return
        setError(err)
        onErrorRef.current?.(err)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, enabled, tick])

  return { data, loading, error, refetch }
}
