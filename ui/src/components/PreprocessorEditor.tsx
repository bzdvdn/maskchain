import { useState } from 'react'
import type { PreprocessorDef } from '../api/profiles'

interface Props {
  preprocessors: PreprocessorDef[]
  onChange: (preprocessors: PreprocessorDef[]) => void
}

// @sk-task 41-profiles-ui#T4.2: Inline preprocessor rule editor (AC-007)
export function PreprocessorEditor({ preprocessors, onChange }: Props) {
  const [expanded, setExpanded] = useState(false)

  function addPreprocessor() {
    onChange([
      ...preprocessors,
      { name: '', type: 'csv', rules: [] },
    ])
  }

  function removePreprocessor(index: number) {
    onChange(preprocessors.filter((_, i) => i !== index))
  }

  function updatePreprocessor(index: number, update: Partial<PreprocessorDef>) {
    onChange(
      preprocessors.map((pp, i) => (i === index ? { ...pp, ...update } : pp))
    )
  }

  function addRule(ppIndex: number, rule: PreprocessorDef['rules'][0]) {
    const pp = preprocessors[ppIndex]
    updatePreprocessor(ppIndex, { rules: [...pp.rules, rule] })
  }

  function removeRule(ppIndex: number, ruleIndex: number) {
    const pp = preprocessors[ppIndex]
    updatePreprocessor(ppIndex, {
      rules: pp.rules.filter((_, i) => i !== ruleIndex),
    })
  }

  return (
    <div className="editor-section">
      <button
        type="button"
        className="editor-toggle"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? '▾' : '▸'} Preprocessors ({preprocessors.length})
      </button>

      {expanded && (
        <div className="editor-body">
          {preprocessors.map((pp, i) => (
            <div key={i} className="editor-item card">
              <div className="editor-item-header">
                <input
                  value={pp.name}
                  onChange={(e) =>
                    updatePreprocessor(i, { name: e.target.value })
                  }
                  placeholder="Preprocessor name"
                  className="editor-input"
                />
                <select
                  value={pp.type}
                  onChange={(e) =>
                    updatePreprocessor(i, { type: e.target.value })
                  }
                  className="editor-select"
                >
                  <option value="csv">CSV</option>
                  <option value="json">JSON</option>
                </select>
                <button
                  type="button"
                  className="btn btn-small btn-danger"
                  onClick={() => removePreprocessor(i)}
                >
                  Remove
                </button>
              </div>

              <div className="rules-list">
                {pp.rules.map((rule, j) => (
                  <div key={j} className="rule-row">
                    <span className="rule-detail">
                      {rule.columns
                        ? `columns: ${rule.columns.join(', ')}`
                        : `path: ${rule.path}`}
                      {' → '}
                      {rule.mask}
                    </span>
                    <button
                      type="button"
                      className="btn-small"
                      onClick={() => removeRule(i, j)}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>

              <RuleInput
                type={pp.type}
                onAdd={(rule) => addRule(i, rule)}
              />
            </div>
          ))}

          <button
            type="button"
            className="btn btn-small"
            onClick={addPreprocessor}
          >
            + Add Preprocessor
          </button>
        </div>
      )}
    </div>
  )
}

function RuleInput({
  type,
  onAdd,
}: {
  type: string
  onAdd: (rule: PreprocessorDef['rules'][0]) => void
}) {
  const [columns, setColumns] = useState('')
  const [path, setPath] = useState('')
  const [mask, setMask] = useState<'full' | 'surname'>('full')

  function handleAdd() {
    const rule: PreprocessorDef['rules'][0] = { mask }
    if (type === 'csv' && columns.trim()) {
      rule.columns = columns.split(',').map((c) => c.trim())
    }
    if (type === 'json' && path.trim()) {
      rule.path = path.trim()
    }
    if (!rule.columns && !rule.path) return
    onAdd(rule)
    setColumns('')
    setPath('')
  }

  return (
    <div className="rule-input-row">
      {type === 'csv' ? (
        <input
          value={columns}
          onChange={(e) => setColumns(e.target.value)}
          placeholder="Column names (comma separated)"
          className="editor-input"
        />
      ) : (
        <input
          value={path}
          onChange={(e) => setPath(e.target.value)}
          placeholder="JSON path (e.g. $.email)"
          className="editor-input"
        />
      )}
      <select
        value={mask}
        onChange={(e) => setMask(e.target.value as 'full' | 'surname')}
        className="editor-select"
      >
        <option value="full">Full mask</option>
        <option value="surname">Surname mask</option>
      </select>
      <button type="button" className="btn-small" onClick={handleAdd}>
        Add Rule
      </button>
    </div>
  )
}
