import { useState } from 'react'
import type { DictionaryDTO } from '../api/profiles'

interface Props {
  dictionaries: DictionaryDTO[]
  onChange: (dictionaries: DictionaryDTO[]) => void
}

// @sk-task 41-profiles-ui#T4.1: Inline dictionary entries editor (AC-006)
export function DictionaryEditor({ dictionaries, onChange }: Props) {
  const [expanded, setExpanded] = useState(false)

  function addDictionary() {
    onChange([
      ...dictionaries,
      { name: '', entries: [], match_mode: 'exact' },
    ])
  }

  function removeDictionary(index: number) {
    onChange(dictionaries.filter((_, i) => i !== index))
  }

  function updateDictionary(index: number, update: Partial<DictionaryDTO>) {
    onChange(
      dictionaries.map((d, i) => (i === index ? { ...d, ...update } : d))
    )
  }

  function addEntry(dictIndex: number, entry: string) {
    const d = dictionaries[dictIndex]
    updateDictionary(dictIndex, { entries: [...d.entries, entry] })
  }

  function removeEntry(dictIndex: number, entryIndex: number) {
    const d = dictionaries[dictIndex]
    updateDictionary(dictIndex, {
      entries: d.entries.filter((_, i) => i !== entryIndex),
    })
  }

  return (
    <div className="editor-section">
      <button
        type="button"
        className="editor-toggle"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? '▾' : '▸'} Dictionaries ({dictionaries.length})
      </button>

      {expanded && (
        <div className="editor-body">
          {dictionaries.map((dict, i) => (
            <div key={i} className="editor-item card">
              <div className="editor-item-header">
                <input
                  value={dict.name}
                  onChange={(e) => updateDictionary(i, { name: e.target.value })}
                  placeholder="Dictionary name"
                  className="editor-input"
                />
                <select
                  value={dict.match_mode}
                  onChange={(e) =>
                    updateDictionary(i, {
                      match_mode: e.target.value as DictionaryDTO['match_mode'],
                    })
                  }
                  className="editor-select"
                >
                  <option value="exact">Exact</option>
                  <option value="contains">Contains</option>
                  <option value="regex">Regex</option>
                  <option value="fuzzy">Fuzzy</option>
                </select>
                <button
                  type="button"
                  className="btn btn-small btn-danger"
                  onClick={() => removeDictionary(i)}
                >
                  Remove
                </button>
              </div>

              <div className="entries-list">
                {dict.entries.map((entry, j) => (
                  <div key={j} className="entry-row">
                    <span className="entry-value">{entry}</span>
                    <button
                      type="button"
                      className="btn-small"
                      onClick={() => removeEntry(i, j)}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>

              <EntryInput onAdd={(entry) => addEntry(i, entry)} />
            </div>
          ))}

          <button
            type="button"
            className="btn btn-small"
            onClick={addDictionary}
          >
            + Add Dictionary
          </button>
        </div>
      )}
    </div>
  )
}

function EntryInput({ onAdd }: { onAdd: (entry: string) => void }) {
  const [value, setValue] = useState('')

  function handleAdd() {
    const trimmed = value.trim()
    if (!trimmed) return
    onAdd(trimmed)
    setValue('')
  }

  return (
    <div className="entry-input-row">
      <input
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            handleAdd()
          }
        }}
        placeholder="Add entry..."
        className="editor-input"
      />
      <button type="button" className="btn-small" onClick={handleAdd}>
        Add
      </button>
    </div>
  )
}
