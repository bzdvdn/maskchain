import { useRef, useState, type KeyboardEvent } from 'react'

interface ChipProps {
  children: React.ReactNode
  onRemove?: () => void
}

export function ChipTag({ children, onRemove }: ChipProps) {
  return (
    <span className="mc-chip">
      {children}
      {onRemove && (
        <button
          type="button"
          className="mc-chip-remove"
          aria-label={`Remove ${children}`}
          onClick={onRemove}
        >
          &times;
        </button>
      )}
    </span>
  )
}

interface ChipInputProps {
  value: string[]
  onChange: (value: string[]) => void
  placeholder?: string
  max?: number
}

export function ChipInput({ value, onChange, placeholder, max }: ChipInputProps) {
  const [draft, setDraft] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const commit = () => {
    const item = draft.trim()
    setDraft('')
    if (!item) return
    if (max !== undefined && value.length >= max) return
    if (value.includes(item)) return
    onChange([...value, item])
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }

  return (
    <div className="mc-chip-input" onClick={() => inputRef.current?.focus()}>
      {value.map((item) => (
        <ChipTag
          key={item}
          onRemove={() => onChange(value.filter((v) => v !== item))}
        >
          {item}
        </ChipTag>
      ))}
      <input
        ref={inputRef}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={commit}
        aria-label={placeholder}
        placeholder={value.length === 0 ? placeholder : undefined}
      />
    </div>
  )
}