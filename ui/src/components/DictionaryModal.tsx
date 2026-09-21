import { Modal } from './ui'
import type { DictionaryItem } from '../api/tenants'

interface Props {
  dict: DictionaryItem
  onClose: () => void
}

export function DictionaryModal({ dict, onClose }: Props) {
  const entries = Array.isArray(dict.entries) ? dict.entries : []

  return (
    <Modal open onClose={onClose} title={dict.name} ariaLabel={dict.name}>
      <p className="modal-meta">
        Match mode: <code>{dict.match_mode}</code> · {entries.length} entries
      </p>
      <table className="modal-table">
        <thead>
          <tr>
            <th>#</th>
            <th>Entry</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e: any, i: number) => (
            <tr key={i}>
              <td className="modal-index">{i + 1}</td>
              <td className="mono">
                {typeof e === 'string' ? e : JSON.stringify(e, null, 2)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Modal>
  )
}
