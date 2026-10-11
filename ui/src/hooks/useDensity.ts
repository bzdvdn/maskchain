import { useCallback, useEffect, useState } from 'react'

export type Density = 'comfortable' | 'compact'

const STORAGE_KEY = 'maskchain-density'

function getInitialDensity(): Density {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'compact' ? 'compact' : 'comfortable'
  } catch {
    return 'comfortable'
  }
}

// @sk-task ui-visual-polish#T1.2: persisted density driving body[data-density] (DEC-003, AC-003)
export function useDensity() {
  const [density, setDensity] = useState<Density>(getInitialDensity)

  useEffect(() => {
    document.body.dataset.density = density
    try {
      localStorage.setItem(STORAGE_KEY, density)
    } catch {
      /* ignore quota errors */
    }
  }, [density])

  const toggleDensity = useCallback(() => {
    setDensity((d) => (d === 'comfortable' ? 'compact' : 'comfortable'))
  }, [])

  return { density, toggleDensity }
}
