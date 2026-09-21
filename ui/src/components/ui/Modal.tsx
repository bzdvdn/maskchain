import { useEffect, useId, useRef, type ReactNode, type RefObject } from 'react'

// @sk-task ui-production-readiness#T2.1: Shared modal primitive (AC-005)
//
// Modal provides the console's single dialog behavior: focus is moved into the
// dialog and trapped there, Escape and backdrop clicks close it, the background
// scroll is locked, and it exposes an accessible name via aria-labelledby.
export interface ModalProps {
  open: boolean
  onClose: () => void
  title?: ReactNode
  ariaLabel?: string
  children: ReactNode
  footer?: ReactNode
  className?: string
  closeOnBackdrop?: boolean
  initialFocusRef?: RefObject<HTMLElement | null>
}

const FOCUSABLE =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function Modal({
  open,
  onClose,
  title,
  ariaLabel,
  children,
  footer,
  className,
  closeOnBackdrop = true,
  initialFocusRef,
}: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const titleId = useId()

  useEffect(() => {
    if (!open) return
    const previouslyFocused = document.activeElement as HTMLElement | null

    const target = initialFocusRef?.current ?? dialogRef.current?.querySelector<HTMLElement>(FOCUSABLE) ?? dialogRef.current
    target?.focus()

    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    function onKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key !== 'Tab' || !dialogRef.current) return
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(FOCUSABLE))
      if (focusable.length === 0) {
        e.preventDefault()
        dialogRef.current.focus()
        return
      }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      const active = document.activeElement
      if (e.shiftKey && (active === first || active === dialogRef.current)) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && active === last) {
        e.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
      previouslyFocused?.focus?.()
    }
  }, [open, onClose, initialFocusRef])

  if (!open) return null

  return (
    <div className="modal-backdrop" onMouseDown={closeOnBackdrop ? onClose : undefined}>
      <div
        ref={dialogRef}
        className={`modal generic-modal${className ? ` ${className}` : ''}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title !== undefined ? titleId : undefined}
        aria-label={title === undefined ? ariaLabel : undefined}
        tabIndex={-1}
        onMouseDown={(e) => e.stopPropagation()}
      >
        {title !== undefined && (
          <div className="modal-header">
            <h3 id={titleId}>{title}</h3>
            <button type="button" className="btn btn-small" onClick={onClose} aria-label="Close">
              ✕
            </button>
          </div>
        )}
        <div className="modal-body">{children}</div>
        {footer && <div className="form-actions">{footer}</div>}
      </div>
    </div>
  )
}
