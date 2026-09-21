import { useEffect, useState } from 'react'
import { getShieldCatalog, type ShieldPack } from '../api/shield'

interface Result {
  packs: ShieldPack[]
  loading: boolean
  error: boolean
}

// useShieldCatalog loads the available compliance packs from the backend
// catalog so pages never hardcode the pack list.
export function useShieldCatalog(): Result {
  const [packs, setPacks] = useState<ShieldPack[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    let mounted = true
    getShieldCatalog()
      .then((catalog) => {
        if (mounted) setPacks(Array.isArray(catalog?.packs) ? catalog.packs : [])
      })
      .catch(() => {
        if (mounted) setError(true)
      })
      .finally(() => {
        if (mounted) setLoading(false)
      })
    return () => {
      mounted = false
    }
  }, [])

  return { packs, loading, error }
}
