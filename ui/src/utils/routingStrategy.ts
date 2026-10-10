import type { ProviderDto, RouteDto } from '../api/routing'

// Strategy taxonomy (mirrors the gateway selector semantics):
//   single   - exactly one provider
//   priority - ≥2 providers in the same minimum priority tier, no weights set
//   weighted - ≥2 healthy providers in the same minimum tier, at least one weight set
//   fallback - healthy providers span more than one priority tier
export type StrategyKind = 'single' | 'priority' | 'weighted' | 'fallback'

export interface StrategyShare {
  provider: string
  pct: number
}

export interface RouteStrategy {
  kind: StrategyKind
  names: string[]
  shares: StrategyShare[]
  healthyCount: number
  unknown: boolean
}

// effectiveWeight mirrors the backend: a missing/zero weight counts as 1.
export function effectiveWeight(weight?: number): number {
  return weight && weight > 0 ? weight : 1
}

export function computeRouteStrategy(route: RouteDto, providers: ProviderDto[]): RouteStrategy {
  const byName = new Map(providers.map((p) => [p.name, p]))
  const names = route.providers ?? []
  const resolved = names.map((name) => ({ name, provider: byName.get(name) }))
  const unknown = resolved.some((r) => !r.provider)

  if (names.length === 0) {
    return { kind: 'single', names, shares: [], healthyCount: 0, unknown }
  }
  if (names.length === 1) {
    const healthy = resolved[0].provider && resolved[0].provider.status !== 'down'
    return { kind: 'single', names, shares: [], healthyCount: healthy ? 1 : 0, unknown }
  }

  const healthy = resolved.filter((r) => r.provider && r.provider.status !== 'down')
  if (healthy.length === 0) {
    return { kind: 'fallback', names, shares: [], healthyCount: 0, unknown }
  }

  const priorities = healthy.map((r) => r.provider!.priority ?? 0)
  const minPriority = Math.min(...priorities)
  const tier = healthy.filter((r) => (r.provider!.priority ?? 0) === minPriority)
  const spansTiers = new Set(priorities).size > 1

  if (spansTiers) {
    return { kind: 'fallback', names, shares: [], healthyCount: healthy.length, unknown }
  }

  const anyWeighted = tier.some((r) => (r.provider!.weight ?? 0) > 0)
  if (anyWeighted) {
    const total = tier.reduce((sum, r) => sum + effectiveWeight(r.provider!.weight), 0)
    const shares = tier.map((r) => ({
      provider: r.name,
      pct: Math.round((effectiveWeight(r.provider!.weight) / total) * 100),
    }))
    return { kind: 'weighted', names, shares, healthyCount: healthy.length, unknown }
  }

  return { kind: 'priority', names, shares: [], healthyCount: healthy.length, unknown }
}
