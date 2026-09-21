import { useEffect, useState } from 'react'
import { getSystemStatus } from '../api/admin'

// The UI is served by the admin process; endpoint snippets must point at the
// data-plane gateway. The admin status endpoint reports its configured
// gateway_url, so fetch it once and share the result.
let cached: string | null = null
let inflight: Promise<string> | null = null

function loadGatewayBase(): Promise<string> {
  if (cached) return Promise.resolve(cached)
  if (!inflight) {
    inflight = getSystemStatus()
      .then((s) => {
        cached = (s.gateway_url && s.gateway_url.replace(/\/+$/, '')) || window.location.origin
        return cached
      })
      .catch(() => {
        cached = window.location.origin
        return cached
      })
      .finally(() => {
        inflight = null
      })
  }
  return inflight
}

// useGatewayBase returns the gateway base URL, falling back to the current origin.
export function useGatewayBase(): string {
  const [base, setBase] = useState<string>(cached ?? '')
  useEffect(() => {
    let mounted = true
    loadGatewayBase().then((b) => {
      if (mounted) setBase(b)
    })
    return () => {
      mounted = false
    }
  }, [])
  return base
}
