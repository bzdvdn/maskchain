import { useEffect, useState } from 'react'

const WORKSPACE_KEY = 'maskchain.workspace'

export function getWorkspace(): string {
  try {
    return localStorage.getItem(WORKSPACE_KEY) ?? ''
  } catch {
    return ''
  }
}

// setWorkspace persists the tenant scope and notifies every subscriber, so any
// page can react to the global workspace switcher.
export function setWorkspace(slug: string) {
  try {
    localStorage.setItem(WORKSPACE_KEY, slug)
  } catch {
    /* localStorage unavailable */
  }
  window.dispatchEvent(new CustomEvent<string>('maskchain:workspace', { detail: slug }))
}

// useWorkspace returns the current tenant scope and a setter shared across pages.
export function useWorkspace(): [string, (slug: string) => void] {
  const [workspace, setLocal] = useState<string>(getWorkspace)

  useEffect(() => {
    const onWorkspace = (e: Event) => setLocal((e as CustomEvent<string>).detail)
    window.addEventListener('maskchain:workspace', onWorkspace)
    return () => window.removeEventListener('maskchain:workspace', onWorkspace)
  }, [])

  return [workspace, setWorkspace]
}
