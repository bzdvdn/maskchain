import { Copy, Check } from 'lucide-react'
import { useCopy } from '../hooks/useCopy'

interface Props {
  text: string
  label?: string
}

export function CopyButton({ text, label }: Props) {
  const { copied, copy } = useCopy()
  return (
    <button
      type="button"
      className="btn-copy"
      title={label ?? 'Copy'}
      aria-label={label ?? `Copy ${text}`}
      onClick={(e) => {
        e.stopPropagation()
        copy(text)
      }}
    >
      {copied ? <Check size={12} /> : <Copy size={12} />}
    </button>
  )
}