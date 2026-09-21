import { Button, Modal } from './ui'

interface Props {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmModal({ open, title, message, confirmLabel = 'Delete', busy = false, onConfirm, onCancel }: Props) {
  return (
    <Modal
      open={open}
      onClose={onCancel}
      title={title}
      footer={
        <>
          <Button onClick={onCancel} disabled={busy}>Cancel</Button>
          <Button variant="danger" onClick={onConfirm} disabled={busy}>{busy ? 'Deleting…' : confirmLabel}</Button>
        </>
      }
    >
      <p className="text-muted" style={{ margin: '0 0 4px' }}>{message}</p>
    </Modal>
  )
}
