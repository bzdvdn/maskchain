import { useEffect, useState } from 'react'
import type { DictionaryItem } from '../api/tenants'

const MATCH_MODES = ['exact', 'contains', 'regex', 'fuzzy'] as const

// @sk-task ui-dictionary-editor#T1.1: entries <-> lines serialization (AC-003)
export function entriesToLines(entries: unknown[]): string {
  return (Array.isArray(entries) ? entries : [])
    .map((e) => (typeof e === 'string' ? e : JSON.stringify(e)))
    .join('\n')
}

export function linesToEntries(text: string): unknown[] {
  return text
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l.length > 0)
    .map((l) => {
      try {
        return JSON.parse(l)
      } catch {
        return l
      }
    })
}

interface Props {
  value: DictionaryItem[]
  onChange: (next: DictionaryItem[]) => void
  errors?: Record<number, string>
}

// @sk-task ui-dictionary-editor#T2.1: controlled tenant dictionary editor (AC-001, AC-002, AC-005)
export function DictionaryEditor({ value, onChange, errors }: Props) {
  const [texts, setTexts] = useState<string[]>(() => value.map((d) => entriesToLines(d.entries)))

  useEffect(() => {
    setTexts((prev) =>
      value.map((d, i) => (prev.length === value.length && prev[i] !== undefined ? prev[i] : entriesToLines(d.entries))),
    )
  }, [value])

  function update(index: number, patch: Partial<DictionaryItem>) {
    onChange(value.map((d, i) => (i === index ? { ...d, ...patch } : d)))
  }

  function add() {
    onChange([...value, { name: '', entries: [], match_mode: 'exact' }])
  }

  function remove(index: number) {
    onChange(value.filter((_, i) => i !== index))
  }

  function setText(index: number, text: string) {
    setTexts((prev) => prev.map((t, i) => (i === index ? text : t)))
    update(index, { entries: linesToEntries(text) })
  }

  return (
    <div className="editor-section">
      <h3 className="h3" style={{ marginBottom: '0.75rem' }}>Dictionaries</h3>

      {value.length === 0 && (
        <p className="muted" style={{ marginBottom: '0.75rem' }}>
          No dictionaries yet. Add one to mask values from a fixed list.
        </p>
      )}

      {value.map((d, i) => (
        <div key={i} className="editor-item card">
          <div className="editor-item-header">
            <input
              className="editor-input"
              aria-label={`Dictionary ${i + 1} name`}
              value={d.name}
              onChange={(e) => update(i, { name: e.target.value })}
              placeholder="Name (e.g. clients)"
            />
            <select
              className="editor-select"
              aria-label={`Dictionary ${i + 1} match mode`}
              value={d.match_mode}
              onChange={(e) => update(i, { match_mode: e.target.value })}
            >
              {MATCH_MODES.map((m) => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
            <button
              type="button"
              className="btn btn-small btn-danger"
              aria-label={`Remove dictionary ${i + 1}`}
              onClick={() => remove(i)}
            >
              ×
            </button>
          </div>

          <textarea
            className="editor-input"
            style={{ width: '100%', minHeight: '72px', resize: 'vertical' }}
            rows={3}
            aria-label={`Dictionary ${i + 1} entries`}
            value={texts[i] ?? ''}
            onChange={(e) => setText(i, e.target.value)}
            placeholder="One entry per line"
          />

          {errors?.[i] && (
            <span className="muted-sm" style={{ display: 'block', marginTop: '0.25rem', color: 'var(--red)' }}>
              {errors[i]}
            </span>
          )}
        </div>
      ))}

      <button type="button" className="btn btn-small" onClick={add}>
        + Add dictionary
      </button>
    </div>
  )
}
