import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  createProfile,
  updateProfile,
  getProfile,
  type CreateProfileRequest,
  type UpdateProfileRequest,
  type DictionaryDTO,
  type PreprocessorDef,
  type ErrorResponse,
} from '../../api/profiles'
import { DictionaryEditor } from '../../components/DictionaryEditor'
import { PreprocessorEditor } from '../../components/PreprocessorEditor'

// @sk-task 41-profiles-ui#T2.3 + T3.2: Profile create/edit form with validation + delete (AC-004, AC-005, AC-008)

interface FormData {
  name: string
  slug: string
  description: string
  dictionaries: DictionaryDTO[]
  preprocessors: PreprocessorDef[]
}

interface FormErrors {
  name?: string
  slug?: string
  server?: string
}

export function ProfileForm() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const isEdit = Boolean(slug)

  const [form, setForm] = useState<FormData>({
    name: '',
    slug: '',
    description: '',
    dictionaries: [],
    preprocessors: [],
  })
  const [errors, setErrors] = useState<FormErrors>({})
  const [submitting, setSubmitting] = useState(false)
  const [loading, setLoading] = useState(isEdit)

  useEffect(() => {
    if (!slug) return
    setLoading(true)
    getProfile(slug)
      .then((p) =>
        setForm({
          name: p.name,
          slug: p.slug,
          description: p.description || '',
          dictionaries: p.dictionaries || [],
          preprocessors: p.preprocessors || [],
        })
      )
      .catch(() => navigate('/profiles'))
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
    setErrors(e)
    return Object.keys(e).length === 0
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!validate()) return

    setSubmitting(true)
    setErrors({})

    try {
      if (isEdit && slug) {
        const req: UpdateProfileRequest = {
          name: form.name,
          description: form.description || undefined,
          dictionaries: form.dictionaries,
          preprocessors: form.preprocessors,
        }
        await updateProfile(slug, req)
        navigate(`/profiles/${slug}`)
      } else {
        const req: CreateProfileRequest = {
          slug: form.slug,
          name: form.name,
          description: form.description || undefined,
          dictionaries: form.dictionaries,
          preprocessors: form.preprocessors,
        }
        const created = await createProfile(req)
        navigate(`/profiles/${created.slug}`)
      }
    } catch (err: any) {
      if (err.name === 'ApiError' && err.body) {
        const apiErr = err as { status: number; body: ErrorResponse }
        if (apiErr.status === 409) {
          setErrors({ server: 'A profile with this slug already exists.' })
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
        setErrors({ server: 'Failed to save profile. Please try again.' })
      }
    } finally {
      setSubmitting(false)
    }
  }

  function setField<K extends keyof FormData>(key: K, value: FormData[K]) {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  if (loading) return <div className="loading">Loading...</div>

  return (
    <div className="profile-form">
      <h1>{isEdit ? 'Edit Profile' : 'Create Profile'}</h1>

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
          <label htmlFor="slug">
            {isEdit ? 'Slug (read-only)' : 'Slug *'}
          </label>
          {isEdit ? (
            <span className="slug-readonly">{form.slug}</span>
          ) : (
            <>
              <input
                id="slug"
                value={form.slug}
                onChange={(e) => setField('slug', e.target.value)}
                className={errors.slug ? 'input-error' : ''}
                placeholder="my-profile"
              />
              {errors.slug && (
                <span className="field-error">{errors.slug}</span>
              )}
            </>
          )}
        </div>

        <div className="form-field">
          <label htmlFor="description">Description</label>
          <textarea
            id="description"
            value={form.description}
            onChange={(e) => setField('description', e.target.value)}
            rows={3}
          />
        </div>

        <DictionaryEditor
          dictionaries={form.dictionaries}
          onChange={(v: DictionaryDTO[]) => setField('dictionaries', v)}
        />

        <PreprocessorEditor
          preprocessors={form.preprocessors}
          onChange={(v: PreprocessorDef[]) => setField('preprocessors', v)}
        />

        <div className="form-actions">
          <button
            type="button"
            className="btn"
            onClick={() => navigate('/profiles')}
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
