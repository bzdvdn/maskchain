import { useEffect, useState } from 'react'
import { apiFetch, UnauthorizedError } from '../api/client'

interface Props {
  children: React.ReactNode
  onUnauthorized: () => void
}

export function ProtectedRoute({ children, onUnauthorized }: Props) {
  const [checking, setChecking] = useState(true)

  useEffect(() => {
    // Verify the session is still valid by calling a lightweight endpoint
    apiFetch('/api/v1/tenants', { method: 'HEAD' })
      .then(() => setChecking(false))
      .catch((err) => {
        if (err instanceof UnauthorizedError) {
          onUnauthorized()
        } else {
          // Server error — allow through, maybe it's transient
          setChecking(false)
        }
      })
  }, [onUnauthorized])

  if (checking) {
    return <div className="loading">Verifying session...</div>
  }

  return <>{children}</>
}
