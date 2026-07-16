import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  createTenant,
  updateTenant,
  getTenant,
  type CreateTenantRequest,
  type UpdateTenantRequest,
  type DictionaryItem,
  type PIIConfig,
  type PIARule,
  type ErrorResponse,
} from '../../api/tenants'

interface FormData {
  name: string
  slug: string
  auth_header: string
  api_keys: string
  dictionaries: DictionaryItem[]
  pii_enabled: boolean
  pii_default_action: string
  pii_rules: PIARule[]
}

interface FormErrors {
  name?: string
  slug?: string
  api_keys?: string
  server?: string
}

const emptyPIIRule = (): PIARule => ({ label: '', type: 'regex', pattern: '', action: 'block' })

export function TenantForm() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const isEdit = Boolean(slug)

  const [form, setForm] = useState<FormData>({
    name: '',
    slug: '',
    auth_header: 'Authorization',
    api_keys: '',
    dictionaries: [],
    pii_enabled: false,
    pii_default_action: 'mask',
    pii_rules: [],
  })
  const [errors, setErrors] = useState<FormErrors>({})
  const [submitting, setSubmitting] = useState(false)
  const [loading, setLoading] = useState(isEdit)

  useEffect(() => {
    if (!slug) return
    setLoading(true)
    getTenant(slug)
      .then((t) =>
        setForm({
          name: t.name,
          slug: t.slug,
          auth_header: t.auth_header,
          api_keys: t.api_keys.join(', '),
          dictionaries: t.dictionaries || [],
          pii_enabled: t.pii_config?.enabled ?? false,
          pii_default_action: t.pii_config?.default_action ?? 'mask',
          pii_rules: t.pii_config?.rules ?? [],
        })
      )
      .catch(() => navigate('/tenants'))
      .finally(() => setLoading(false))
  }, [slug, navigate])

  function validate(): boolean {
    const e: FormErrors = {}
    if (!form.name.trim()) e.name = 'Name is required'
    if (!isEdit) {
      if (!form.slug.trim()) {
        e.slug = 'Slug is required'
      } else if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(form.slug)) {
        e.slug = 'Slug must be lowercase alphanumeric with hyphens'
      }
    }
    if (!form.api_keys.trim()) e.api_keys = 'At least one API key is required'
    setErrors(e)
    return Object.keys(e).length === 0
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!validate()) return

    setSubmitting(true)
    setErrors({})

    const keys = form.api_keys.split(',').map((k) => k.trim()).filter(Boolean)
    const piiCfg: PIIConfig | undefined = form.pii_enabled || form.pii_rules.length > 0
      ? {
          enabled: form.pii_enabled,
          default_action: form.pii_default_action,
          rules: form.pii_rules,
        }
      : undefined

    try {
      if (isEdit && slug) {
        const req: UpdateTenantRequest = {
          name: form.name,
          auth_header: form.auth_header || undefined,
          api_keys: keys,
          dictionaries: form.dictionaries,
          pii_config: piiCfg,
        }
        await updateTenant(slug, req)
        navigate(`/tenants/${slug}`)
      } else {
        const req: CreateTenantRequest = {
          slug: form.slug,
          name: form.name,
          auth_header: form.auth_header || undefined,
          api_keys: keys,
          dictionaries: form.dictionaries,
          pii_config: piiCfg,
        }
        const created = await createTenant(req)
        navigate(`/tenants/${created.slug}`)
      }
    } catch (err: any) {
      if (err.name === 'ApiError' && err.body) {
        const apiErr = err as { status: number; body: ErrorResponse }
        if (apiErr.status === 409) {
          setErrors({ server: 'A tenant with this slug already exists.' })
        } else if (apiErr.body.details) {
          const serverErrors: FormErrors = {}
          for (const d of apiErr.body.details) {
            const field = d.field.toLowerCase()
            if (field === 'name') serverErrors.name = d.message
            if (field === 'slug') serverErrors.slug = d.message
          }
          setErrors(serverErrors)
        } else {
          setErrors({ server: apiErr.body.error })
        }
      } else {
        setErrors({ server: 'Failed to save tenant. Please try again.' })
      }
    } finally {
      setSubmitting(false)
    }
  }

  function setField<K extends keyof FormData>(key: K, value: FormData[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  function updatePIIRule(index: number, update: Partial<PIARule>) {
    setField('pii_rules', form.pii_rules.map((r, i) => i === index ? { ...r, ...update } : r))
  }

  function addPIIRule() {
    setField('pii_rules', [...form.pii_rules, emptyPIIRule()])
  }

  function removePIIRule(index: number) {
    setField('pii_rules', form.pii_rules.filter((_, i) => i !== index))
  }

  if (loading) return <div className="loading">Loading...</div>

  return (
    <div className="profile-form">
      <h1>{isEdit ? 'Edit Tenant' : 'Create Tenant'}</h1>

      {errors.server && <div className="error-banner">{errors.server}</div>}

      <form onSubmit={handleSubmit}>
        <div className="form-field">
          <label htmlFor="name">Name *</label>
          <input
            id="name"
            value={form.name}
            onChange={(e) => setField('name', e.target.value)}
            className={errors.name ? 'input-error' : ''}
          />
          {errors.name && <span className="field-error">{errors.name}</span>}
        </div>

        <div className="form-field">
          <label htmlFor="slug">{isEdit ? 'Slug (read-only)' : 'Slug *'}</label>
          {isEdit ? (
            <span className="slug-readonly">{form.slug}</span>
          ) : (
            <>
              <input
                id="slug"
                value={form.slug}
                onChange={(e) => setField('slug', e.target.value)}
                className={errors.slug ? 'input-error' : ''}
                placeholder="my-tenant"
              />
              {errors.slug && <span className="field-error">{errors.slug}</span>}
            </>
          )}
        </div>

        <div className="form-field">
          <label htmlFor="auth_header">Auth Header</label>
          <input
            id="auth_header"
            value={form.auth_header}
            onChange={(e) => setField('auth_header', e.target.value)}
            placeholder="Authorization"
          />
        </div>

        <div className="form-field">
          <label htmlFor="api_keys">
            API Keys * (comma-separated)
          </label>
          <textarea
            id="api_keys"
            value={form.api_keys}
            onChange={(e) => setField('api_keys', e.target.value)}
            rows={2}
            placeholder="sk-key-1, sk-key-2"
            className={errors.api_keys ? 'input-error' : ''}
          />
          {errors.api_keys && <span className="field-error">{errors.api_keys}</span>}
        </div>

        <div className="editor-section">
          <h3 className="h3" style={{ marginBottom: '0.75rem' }}>PII Config</h3>

          <div className="form-field">
            <label>
              <input
                type="checkbox"
                checked={form.pii_enabled}
                onChange={(e) => setField('pii_enabled', e.target.checked)}
                style={{ marginRight: '0.5rem' }}
              />
              PII Scanning Enabled
            </label>
          </div>

          <div className="form-field">
            <label htmlFor="pii_default_action">Default Action</label>
            <select
              id="pii_default_action"
              value={form.pii_default_action}
              onChange={(e) => setField('pii_default_action', e.target.value)}
              className="editor-select"
            >
              <option value="mask">Mask</option>
              <option value="block">Block</option>
              <option value="allow">Allow</option>
            </select>
          </div>

          <div className="editor-body" style={{ border: '1px solid var(--color-border)', borderRadius: '6px', padding: '1rem', marginTop: '0.5rem' }}>
            {form.pii_rules.map((rule, i) => (
              <div key={i} className="editor-item card" style={{ marginBottom: '0.75rem' }}>
                <div className="editor-item-header">
                  <input
                    value={rule.label}
                    onChange={(e) => updatePIIRule(i, { label: e.target.value })}
                    placeholder="Label (e.g. email)"
                    className="editor-input"
                  />
                  <select
                    value={rule.type}
                    onChange={(e) => updatePIIRule(i, { type: e.target.value })}
                    className="editor-select"
                  >
                    <option value="regex">Regex</option>
                    <option value="pii">PII</option>
                    <option value="financial">Financial</option>
                    <option value="secrets">Secrets</option>
                  </select>
                  <input
                    value={rule.pattern}
                    onChange={(e) => updatePIIRule(i, { pattern: e.target.value })}
                    placeholder="Pattern"
                    className="editor-input"
                  />
                  <select
                    value={rule.action}
                    onChange={(e) => updatePIIRule(i, { action: e.target.value })}
                    className="editor-select"
                  >
                    <option value="block">Block</option>
                    <option value="allow">Allow</option>
                  </select>
                  <button
                    type="button"
                    className="btn btn-small btn-danger"
                    onClick={() => removePIIRule(i)}
                  >
                    ×
                  </button>
                </div>
              </div>
            ))}
            <button type="button" className="btn btn-small" onClick={addPIIRule}>
              + Add Rule
            </button>
          </div>
        </div>

        <div className="form-actions">
          <button
            type="button"
            className="btn"
            onClick={() => navigate('/tenants')}
          >
            Cancel
          </button>
          <button
            type="submit"
            className="btn btn-primary"
            disabled={submitting}
          >
            {submitting ? 'Saving...' : 'Save'}
          </button>
        </div>
      </form>
    </div>
  )
}
