// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { computeRouteStrategy, effectiveWeight } from './routingStrategy'
import type { ProviderDto, RouteDto } from '../api/routing'

function provider(name: string, priority = 0, weight = 0, status = 'up'): ProviderDto {
  return { name, api_type: 'openai', base_url: 'http://x', priority, weight, status }
}
function route(model: string, providers: string[]): RouteDto {
  return { tenant: '*', model, providers }
}

// @sk-test operator-console-v2-1#T3.2: strategy taxonomy (AC-005)
describe('computeRouteStrategy', () => {
  it('labels a single provider', () => {
    const s = computeRouteStrategy(route('m', ['a']), [provider('a')])
    expect(s.kind).toBe('single')
  })

  it('labels a same-tier unweighted route as priority', () => {
    const s = computeRouteStrategy(route('m', ['a', 'b']), [provider('a'), provider('b')])
    expect(s.kind).toBe('priority')
  })

  it('labels a same-tier weighted route as weighted', () => {
    const s = computeRouteStrategy(route('m', ['a', 'b']), [provider('a', 0, 3), provider('b', 0, 1)])
    expect(s.kind).toBe('weighted')
  })

  it('labels a route spanning priority tiers as fallback', () => {
    const s = computeRouteStrategy(route('m', ['a', 'b']), [provider('a', 1), provider('b', 5)])
    expect(s.kind).toBe('fallback')
  })
})

// @sk-test operator-console-v2-1#T3.2: weighted distribution from effective weights (AC-006)
describe('weighted distribution', () => {
  it('computes 3:1 as 75% / 25%', () => {
    const s = computeRouteStrategy(route('m', ['a', 'b']), [provider('a', 0, 3), provider('b', 0, 1)])
    expect(s.shares).toEqual([
      { provider: 'a', pct: 75 },
      { provider: 'b', pct: 25 },
    ])
  })

  it('treats a zero weight as 1', () => {
    expect(effectiveWeight(0)).toBe(1)
    expect(effectiveWeight(undefined)).toBe(1)
    expect(effectiveWeight(4)).toBe(4)
  })

  it('excludes unhealthy providers from the tier', () => {
    const s = computeRouteStrategy(route('m', ['a', 'b', 'c']), [provider('a', 0, 3), provider('b', 0, 1), provider('c', 0, 0, 'down')])
    expect(s.healthyCount).toBe(2)
    expect(s.shares).toEqual([
      { provider: 'a', pct: 75 },
      { provider: 'b', pct: 25 },
    ])
  })
})
