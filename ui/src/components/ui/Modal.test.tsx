// @vitest-environment jsdom
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Modal } from './Modal'

// @sk-test ui-production-readiness#T5.3: Shared modal a11y (AC-005)
describe('Modal', () => {
  it('moves focus into the dialog and exposes an accessible name', () => {
    render(
      <Modal open onClose={() => {}} title="Edit provider">
        <button>Save</button>
      </Modal>,
    )

    const dialog = screen.getByRole('dialog')
    const labelId = dialog.getAttribute('aria-labelledby')
    expect(labelId).toBeTruthy()
    expect(document.getElementById(labelId!)?.textContent).toBe('Edit provider')
    expect(dialog.contains(document.activeElement)).toBe(true)
  })

  it('closes on Escape', () => {
    const onClose = vi.fn()
    render(
      <Modal open onClose={onClose} title="Confirm">
        <button>Ok</button>
      </Modal>,
    )

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })

  it('does not render when closed', () => {
    render(
      <Modal open={false} onClose={() => {}} title="Hidden">
        <button>Ok</button>
      </Modal>,
    )
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})
