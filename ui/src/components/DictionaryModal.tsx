import { useEffect, useRef } from 'react'
import type { DictionaryItem } from '../api/tenants'

interface Props {
  dict: DictionaryItem
  onClose: () => void
}

export function DictionaryModal({ dict, onClose }: Props) {
  const backdropRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [onClose])

  function handleBackdrop(e: React.MouseEvent) {
    if (e.target === backdropRef.current) onClose()
  }

  const entries = Array.isArray(dict.entries) ? dict.entries : []

  return (
    <div className="modal-backdrop" ref={backdropRef} onClick={handleBackdrop}>
      <div className="modal">
        <div className="modal-header">
          <h3>{dict.name}</h3>
          <button type="button" className="btn btn-small" onClick={onClose}>
            ✕
          </button>
        </div>
        <div className="modal-body">
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
        </div>
      </div>
    </div>
  )
}
