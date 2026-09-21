import { apiFetch } from './client'

export interface ShieldPack {
  key: string
  name: string
}

export interface ShieldCatalog {
  detectors: string[]
  reactions: string[]
  packs: ShieldPack[]
}

// getShieldCatalog returns the detectors, reactions and compliance packs the
// backend actually serves, so the UI does not hardcode the pack list.
export function getShieldCatalog(): Promise<ShieldCatalog> {
  return apiFetch<ShieldCatalog>('/api/v1/shield/catalog')
}
