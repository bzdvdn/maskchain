import { useEffect, useRef } from 'react'

// @sk-task ui-production-readiness#T6.3: Low-cadence auto-refresh (AC-010)
//
// useAutoRefresh re-runs the callback on a fixed cadence, pausing while the tab
// is hidden so background tabs do not keep polling.
export function useAutoRefresh(fn: () => void, seconds = 30) {
  const fnRef = useRef(fn)
  fnRef.current = fn

  useEffect(() => {
    const id = setInterval(() => {
      if (!document.hidden) fnRef.current()
    }, seconds * 1000)
    return () => clearInterval(id)
  }, [seconds])
}
