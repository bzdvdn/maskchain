// @vitest-environment jsdom
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PageHeader } from './PageHeader'

describe('PageHeader', () => {
  it('renders title, subtitle and actions', () => {
    render(
      <PageHeader title="Virtual keys" subtitle="Manage API keys" actions={<button>Create</button>} />,
    )
    expect(screen.getByRole('heading', { level: 1, name: 'Virtual keys' })).toBeTruthy()
    expect(screen.getByText('Manage API keys')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Create' })).toBeTruthy()
  })
})