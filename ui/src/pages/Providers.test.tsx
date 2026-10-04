// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { Providers } from './Providers'

vi.mock('../api/routing', () => ({
  listProviders: vi.fn(),
  listModels: vi.fn(),
  listProviderModels: vi.fn(),
  deleteProvider: vi.fn(),
  deleteProviderModel: vi.fn(),
  upsertProvider: vi.fn(),
  isMaskedKey: (v?: string) => !!v && v.includes('***'),
}))
vi.mock('../components/Toast', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../components/Toast')>()
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
    useToast: () => ({ toast: vi.fn() }),
  }
})

import { listProviders, listModels, listProviderModels, upsertProvider, deleteProviderModel } from '../api/routing'

const mockProviders = vi.mocked(listProviders)
const mockModels = vi.mocked(listModels)
const mockProviderModels = vi.mocked(listProviderModels)
const mockUpsert = vi.mocked(upsertProvider)
const mockDeleteProviderModel = vi.mocked(deleteProviderModel)

beforeEach(() => {
  vi.clearAllMocks()
  mockModels.mockResolvedValue([
    { model: 'gpt-4o', input_price_per_1k: 0, output_price_per_1k: 0, currency: 'USD', default_providers: ['openrouter'], override_count: 0 },
  ])
})

// @sk-test routing-ia#T4.3: editing a provider with a masked key saves without re-entry (AC-005)
describe('Providers masked secret', () => {
  it('allows saving an openai provider whose key is masked', async () => {
    mockProviders.mockResolvedValue([
      { name: 'openrouter', api_type: 'openai', base_url: 'https://openrouter.ai/api/v1', api_keys: ['sk-***xyz'], status: 'up' },
    ])
    render(<Providers />)

    fireEvent.click(await screen.findByRole('button', { name: 'Edit' }))
    // change only the proxy, keep the masked key
    fireEvent.change(screen.getByPlaceholderText(/corp-proxy/), { target: { value: 'http://corp-proxy:3128' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockUpsert).toHaveBeenCalledTimes(1))
    const payload = mockUpsert.mock.calls[0][0]
    expect(payload.proxy_url).toBe('http://corp-proxy:3128')
    expect(payload.api_keys).toEqual(['sk-***xyz'])
  })
})

// @sk-test routing-ia#T4.3: validation is api_type-aware (AC-005)
describe('Providers validation', () => {
  it('requires a key for openai and accepts ollama without one', async () => {
    mockProviders.mockResolvedValue([])
    render(<Providers />)

    fireEvent.click((await screen.findAllByRole('button', { name: 'Add Provider' }))[0])
    fireEvent.change(screen.getByPlaceholderText('openrouter'), { target: { value: 'p1' } })
    fireEvent.change(screen.getByPlaceholderText('https://openrouter.ai/api/v1'), { target: { value: 'https://x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByText('Add at least one API key')).toBeTruthy()
    expect(mockUpsert).not.toHaveBeenCalled()

    // switch to ollama: no key needed
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'ollama' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(mockUpsert).toHaveBeenCalledTimes(1))
    expect(mockUpsert.mock.calls[0][0].api_type).toBe('ollama')
  })
})

function setApiKey(value: string) {
  const label = screen.getByText('API Keys (comma separated)')
  const input = label.parentElement?.querySelector('input') as HTMLInputElement
  fireEvent.change(input, { target: { value } })
}

// @sk-test routing-ia#T5.3: model picker loads from the provider (AC-012)
describe('Providers model picker', () => {
  it('loads models from the provider and saves the selected ones', async () => {
    mockProviders.mockResolvedValue([])
    mockProviderModels.mockResolvedValue(['gpt-4o', 'llama3.2'])
    render(<Providers />)

    fireEvent.click((await screen.findAllByRole('button', { name: 'Add Provider' }))[0])
    fireEvent.change(screen.getByPlaceholderText('openrouter'), { target: { value: 'openrouter' } })
    fireEvent.change(screen.getByPlaceholderText('https://openrouter.ai/api/v1'), { target: { value: 'https://openrouter.ai/api/v1' } })
    fireEvent.click(screen.getByRole('button', { name: 'Load models from provider' }))

    const checkbox = await screen.findByRole('checkbox', { name: /gpt-4o/ })
    fireEvent.click(checkbox)
    setApiKey('sk-x')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockUpsert).toHaveBeenCalledTimes(1))
    expect(mockProviderModels).toHaveBeenCalledWith('openrouter')
    expect(mockUpsert.mock.calls[0][0].models).toEqual(['gpt-4o'])
  })

  it('falls back to manual entry when loading fails', async () => {
    mockProviders.mockResolvedValue([])
    mockProviderModels.mockRejectedValue(new Error('boom'))
    render(<Providers />)

    fireEvent.click((await screen.findAllByRole('button', { name: 'Add Provider' }))[0])
    fireEvent.change(screen.getByPlaceholderText('openrouter'), { target: { value: 'p1' } })
    fireEvent.change(screen.getByPlaceholderText('https://openrouter.ai/api/v1'), { target: { value: 'https://x' } })
    fireEvent.click(screen.getByRole('button', { name: 'Load models from provider' }))

    expect(await screen.findByText(/Could not load models/)).toBeTruthy()

    const manual = screen.getByPlaceholderText('press Enter to add a model id manually')
    fireEvent.change(manual, { target: { value: 'manual-model' } })
    fireEvent.keyDown(manual, { key: 'Enter' })
    setApiKey('sk-x')
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockUpsert).toHaveBeenCalledTimes(1))
    expect(mockUpsert.mock.calls[0][0].models).toEqual(['manual-model'])
  })
})

// @sk-test provider-model-registry#T3.5: removing a model from a provider calls the catalog endpoint (AC-008)
describe('Providers model removal', () => {
  it('removes a model from the provider card', async () => {
    mockProviders.mockResolvedValue([
      { name: 'openrouter', api_type: 'openai', base_url: 'https://openrouter.ai/api/v1', api_keys: ['sk-x'], status: 'up' },
    ])
    mockDeleteProviderModel.mockResolvedValue(undefined)
    render(<Providers />)

    fireEvent.click(await screen.findByRole('button', { name: 'Remove gpt-4o' }))
    await waitFor(() => expect(mockDeleteProviderModel).toHaveBeenCalledWith('openrouter', 'gpt-4o'))
  })
})

// @sk-test model-aliases-weighted-lb#T3.5: provider weight is saved (AC-009)
describe('Providers weight', () => {
  it('saves the configured weight', async () => {
    mockProviders.mockResolvedValue([])
    render(<Providers />)

    fireEvent.click((await screen.findAllByRole('button', { name: 'Add Provider' }))[0])
    fireEvent.change(screen.getByPlaceholderText('openrouter'), { target: { value: 'p1' } })
    fireEvent.change(screen.getByPlaceholderText('https://openrouter.ai/api/v1'), { target: { value: 'https://x' } })
    setApiKey('sk-x')
    const weightInput = screen.getByText('Weight').parentElement?.querySelector('input') as HTMLInputElement
    fireEvent.change(weightInput, { target: { value: '3' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(mockUpsert).toHaveBeenCalledTimes(1))
    expect(mockUpsert.mock.calls[0][0].weight).toBe(3)
  })
})
