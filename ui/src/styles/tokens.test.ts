import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (name: string) => readFileSync(fileURLToPath(new URL(name, import.meta.url)), 'utf8')
const tokens = read('./tokens.css')
const base = read('./base.css')
const components = read('./components.css')

// @sk-test ui-visual-polish#T1.3: design-token contract (AC-003, AC-009)
describe('design tokens', () => {
  it('exposes semantic, layout and density tokens', () => {
    const required = [
      '--on-accent',
      '--success',
      '--warning',
      '--danger',
      '--info',
      '--focus-ring',
      '--content-max',
      '--sidebar-w-collapsed',
      '--density-row-y',
      '--shadow-lg',
    ]
    for (const t of required) expect(tokens).toContain(t)
  })

  it('completes the light theme with semantic colors', () => {
    const idx = tokens.indexOf("body[data-theme='light']")
    expect(idx).toBeGreaterThan(-1)
    const light = tokens.slice(idx)
    for (const t of ['--success', '--danger', '--warning', '--info', '--shadow-lg']) {
      expect(light).toContain(t)
    }
  })

  it('applies compact density via a body attribute', () => {
    expect(tokens).toContain("body[data-density='compact']")
  })

  it('keeps hardcoded colors out of base/components', () => {
    for (const src of [base, components]) expect(src).not.toMatch(/#[0-9a-fA-F]{3,8}\b/)
  })
})
