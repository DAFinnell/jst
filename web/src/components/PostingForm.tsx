import { useEffect, useRef, useState } from 'react'
import type { ReactNode, SubmitEvent } from 'react'
import { Link } from 'react-router'
import { PostingAPIError } from '../api/postings'
import type {
  EmploymentType,
  Posting,
  PostingFieldErrors,
  PostingInput,
  WorkplaceArrangement,
} from '../api/postings'

type PostingField = keyof PostingInput

type PostingFormProps = {
  initialValues?: PostingInput
  onSave: (
    input: PostingInput,
    signal: AbortSignal,
  ) => Promise<Posting>
  onSaved: (posting: Posting) => void
  onCancel: () => void
}

type FormFeedback = {
  fieldErrors: PostingFieldErrors
  existingPostingId?: number
  message?: string
}

type FocusTarget = {
  field: PostingField | 'general'
}

const emptyInput: PostingInput = {
  company: '',
  title: '',
  url: '',
  description: '',
  location: null,
  employment_type: null,
  workplace_arrangement: null,
}

const fieldOrder: PostingField[] = [
  'company',
  'title',
  'url',
  'location',
  'employment_type',
  'workplace_arrangement',
  'description',
]

const fieldHints: Partial<Record<PostingField, string>> = {
  url: 'Paste the complete URL here.',
  location: 'Blank if the posting does not have a location',
  employment_type: 'Choose unknown if unclear. Other includes contract/part-time roles.',
  workplace_arrangement: 'Choose unknown if unclear.',
  description: 'Paste the description as plain text.',
}

function validateInput(
  input: PostingInput,
): PostingFieldErrors {
  const errors: PostingFieldErrors = {}

  if (!input.company.trim()) {
    errors.company = 'Enter a company.'
  }

  if (!input.title.trim()) {
    errors.title = 'Enter a job title.'
  }

  if (!input.description.trim()) {
    errors.description = 'Enter a job description.'
  }

  const url = input.url.trim()

  if (!url) {
    errors.url = 'Enter a URL.'
  } else if (!/^https?:\/\//i.test(url) || /\s/.test(url)) {
    errors.url = 'Enter a valid URL.'
  } else {
    try {
      const parsed = new URL(url)

      if (
        !parsed.hostname ||
        !['http:', 'https:'].includes(parsed.protocol)
      ) {
        errors.url = 'Enter a valid URL.'
      } else if (parsed.username || parsed.password) {
        errors.url =
          'URLs must not contain username or password.'
      }
    } catch {
      errors.url = 'Enter a valid URL.'
    }
  }
  return errors
}

function Field({
  field,
  label,
  required = false,
  error,
  existingPostingId,
  children,
}: {
  field: PostingField
  label: string
  required?: boolean
  error?: string
  existingPostingId?: number
  children: ReactNode
}) {
  const hint = fieldHints[field]

  return (
    <div className="min-w-0 space-y-2">
      <label
        htmlFor={`posting-${field}`}
        className="block font-medium"
      >
        {label}
        <span className="ml-2 text-sm font-normal text-slate-600">
          ({required ? 'required' : 'optional'})
        </span>
      </label>

      {children}

      {hint && (
        <p
          id={`posting-${field}-hint`}
          className="text-sm text-slate-600"
        >
          {hint}
        </p>
      )}

      {error && (
        <p
          id={`posting-${field}-error`}
          className="text-sm text-red-700"
        >
          {error}
        </p>
      )}

      {existingPostingId !== undefined && (
        <Link
          id="posting-url-duplicate"
          to={`/postings/${existingPostingId}`}
          className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
        >
          View existing posting
        </Link>
      )}
    </div>
  )
}

export default function PostingForm({
  initialValues,
  onSave,
  onSaved,
  onCancel,
}: PostingFormProps) {
  const [values, setValues] = useState<PostingInput>(() => ({
    ...(initialValues ?? emptyInput),
  }))
  const [feedback, setFeedback] = useState<FormFeedback>({
    fieldErrors: {},
  })
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [focusTarget, setFocusTarget] =
    useState<FocusTarget | null>(null)

  const formRef = useRef<HTMLFormElement>(null)
  const summaryRef = useRef<HTMLDivElement>(null)
  const activeRequest = useRef<AbortController | null>(null)

  useEffect(() => {
    return () => {
      activeRequest.current?.abort()
    }
  }, [])

  useEffect(() => {
    if (!focusTarget) {
      return
    }

    if (focusTarget.field === 'general') {
      summaryRef.current?.focus()
      return
    }

    const control =
      formRef.current?.elements.namedItem(focusTarget.field)

    if (control instanceof HTMLElement) {
      control.focus()
    }
  }, [focusTarget])

  function changeField<K extends PostingField>(
    field: K,
    value: PostingInput[K],
  ) {
    setValues((current) => ({
      ...current,
      [field]: value,
    }))

    setFeedback((current) => {
      const fieldErrors = { ...current.fieldErrors }
      delete fieldErrors[field]

      return {
        ...current,
        fieldErrors,
        existingPostingId:
          field === 'url'
            ? undefined
            : current.existingPostingId,
      }
    })
  }

  function showFieldErrors(
    fieldErrors: PostingFieldErrors,
    existingPostingId?: number,
  ) {
    setFeedback({ fieldErrors, existingPostingId })

    const firstField = fieldOrder.find(
      (field) => Boolean(fieldErrors[field])
    )

    setFocusTarget({
      field: firstField ?? 'general',
    })
  }

  function controlProps(field: PostingField) {
    const error = feedback.fieldErrors[field]

    const describedBy = [
      fieldHints[field]
        ? `posting-${field}-hint`
        : undefined,
      error ? `posting-${field}-error` : undefined,
      field === 'url' &&
        feedback.existingPostingId !== undefined
        ? 'posting-url-duplicate'
        : undefined,
    ]
      .filter(Boolean)
      .join(' ')

    return {
      id: `posting-${field}`,
      name: field,
      'aria-invalid': Boolean(error),
      'aria-describedby': describedBy || undefined,
      className:
        'w-full min-w-0 rounded-lg border bg-white px-3 py-2 text-slate-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700 disabled:bg-slate-100 ' +
        (error ? 'border-red-600' : 'border-slate-300'),
    }
  }

  async function handleSubmit(
    event: SubmitEvent<HTMLFormElement>,
  ) {
    event.preventDefault()

    if (activeRequest.current) {
      return
    }

    const fieldErrors = validateInput(values)

    if (Object.keys(fieldErrors).length > 0) {
      showFieldErrors(fieldErrors)
      return
    }

    const input: PostingInput = {
      ...values,
      company: values.company.trim(),
      title: values.title.trim(),
      url: values.url.trim(),
      location: values.location?.trim() || null,
    }

    const controller = new AbortController()
    activeRequest.current = controller

    setFeedback({ fieldErrors: {} })
    setIsSubmitting(true)

    try {
      const saved = await onSave(input, controller.signal)

      if (!controller.signal.aborted) {
        onSaved(saved)
      }
    } catch (error: unknown) {
      if (controller.signal.aborted) {
        return
      }

      if (error instanceof PostingAPIError) {
        if (
          error.status === 400 &&
          error.code === 'validation_failed' &&
          error.fieldErrors &&
          Object.keys(error.fieldErrors).length > 0
        ) {
          showFieldErrors(error.fieldErrors)
          return
        }

        if (
          error.status === 409 &&
          error.code === 'duplicate_url'
        ) {
          showFieldErrors(
            {
              url: 'A posting with this URL already exists.',
            },
            error.existingPostingId,
          )
          return
        }
      }

      const message =
        error instanceof PostingAPIError
          ? error.status === 413
            ? 'This posting is too large to save. Shorten the description and try again.'
            : 'Unable to save this posting. The input is still here. Try Save again.'
          : "Could not confirm the save. Your input is still here. Check Postings before trying to save again."

      setFeedback({ fieldErrors: {}, message })
      setFocusTarget({ field: 'general' })
    } finally {
      if (activeRequest.current === controller) {
        activeRequest.current = null
      }

      if (!controller.signal.aborted) {
        setIsSubmitting(false)
      }
    }
  }

  const hasFieldErrors =
    Object.keys(feedback.fieldErrors).length > 0

  return (
    <form
      ref={formRef}
      noValidate
      onSubmit={handleSubmit}
      className="space-y-5"
    >
      {(hasFieldErrors || feedback.message) && (
        <div
          ref={summaryRef}
          role="alert"
          tabIndex={-1}
          className="rounded-lg border border-red-300 bg-red-50 p-4 text-red-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700"
        >
          {hasFieldErrors
            ? 'Check the highlighted fields.'
            : feedback.message}
        </div>
      )}

      <p
        role="status"
        className="min-h-6 text-slate-600"
      >
        {isSubmitting ? 'Saving...' : ''}
      </p>

      <fieldset
        disabled={isSubmitting}
        aria-busy={isSubmitting}
        className="min-w-0 space-y-5"
      >
        <legend className="sr-only">
          Posting information and actions
        </legend>

        <div className="grid gap-5 sm:grid-cols-2">
          <Field
            field="company"
            label="Company"
            required
            error={feedback.fieldErrors.company}
          >
            <input
              {...controlProps('company')}
              type="text"
              required
              value={values.company}
              onChange={(event) =>
                changeField('company', event.target.value)
              }
            />
          </Field>

          <Field
            field="title"
            label="Title"
            required
            error={feedback.fieldErrors.title}
          >
            <input
              {...controlProps('title')}
              type="text"
              required
              value={values.title}
              onChange={(event) =>
                changeField('title', event.target.value)
              }
            />
          </Field>
        </div>

        <Field
          field="url"
          label="URL"
          required
          error={feedback.fieldErrors.url}
          existingPostingId={feedback.existingPostingId}
        >
          <input
            {...controlProps('url')}
            type="text"
            inputMode="url"
            autoCapitalize="none"
            spellCheck={false}
            required
            value={values.url}
            onChange={(event) =>
              changeField('url', event.target.value)
            }
          />
        </Field>

        <div className="grid gap-5 md:grid-cols-3">
          <Field
            field="location"
            label="Location"
            error={feedback.fieldErrors.location}
          >
            <input
              {...controlProps('location')}
              type="text"
              value={values.location ?? ''}
              onChange={(event) =>
                changeField('location', event.target.value)
              }
            />
          </Field>

          <Field
            field="employment_type"
            label="Employment type"
            error={feedback.fieldErrors.employment_type}
          >
            <select
              {...controlProps('employment_type')}
              value={values.employment_type ?? ''}
              onChange={(event) =>
                changeField(
                  'employment_type',
                  event.target.value === ''
                    ? null
                    : (event.target.value as EmploymentType),
                )
              }
            >
              <option value="">Unknown</option>
              <option value="full_time">Full-time</option>
              <option value="other">Other</option>
            </select>
          </Field>

          <Field
            field="workplace_arrangement"
            label="Workplace arrangement"
            error={feedback.fieldErrors.workplace_arrangement}
          >
            <select
              {...controlProps('workplace_arrangement')}
              value={values.workplace_arrangement ?? ''}
              onChange={(event) =>
                changeField(
                  'workplace_arrangement',
                  event.target.value === ''
                    ? null
                    : (event.target.value as WorkplaceArrangement),
                )
              }
            >
              <option value="">Unknown</option>
              <option value="remote">Remote</option>
              <option value="hybrid">Hybrid</option>
              <option value="on_site">On-site</option>
            </select>
          </Field>
        </div>

        <Field
          field="description"
          label="Description"
          required
          error={feedback.fieldErrors.description}
        >
          <textarea
            {...controlProps('description')}
            rows={12}
            required
            value={values.description}
            onChange={(event) =>
              changeField('description', event.target.value)
            }
          />
        </Field>

        <div className="flex flex-wrap gap-3">
          <button
            type="submit"
            className="rounded-lg bg-slate-900 px-4 py-2 font-medium text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900 disabled:cursor-wait disabled:opacity-60"
          >
            {isSubmitting ? 'Saving…' : 'Save'}
          </button>

          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg border border-slate-300 bg-white px-4 py-2 font-medium hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900 disabled:cursor-wait disabled:opacity-60"
          >
            Cancel
          </button>
        </div>
      </fieldset>
    </form>
  )
}
