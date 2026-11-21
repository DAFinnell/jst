import { useEffect, useRef, useState } from 'react'
import type { SubmitEvent } from 'react'
import {
  getPostingTracking,
  isPostingStage,
  patchPostingTracking,
  PostingAPIError,
  postingStages,
} from '../api/postings'
import type {
  PostingFieldErrors,
  PostingStage,
  PostingStageEvent,
  PostingTrackingData,
  PostingTrackingPatch
} from '../api/postings'
import {
  formatApplicationDate,
  formatPostingDate,
  postingStageLabel,
} from '../postingDisplay'

type TrackingProps = {
  id: string
  disabled: boolean
  onSavingChange: (saving: boolean) => void
  onMissing: () => void
}

type TrackingLoadState =
  | { status: 'loading' }
  | { status: 'success'; tracking: PostingTrackingData }
  | { status: 'error' }

type TrackingDraft = {
  stage: PostingStage
  date: string
}

type TrackingFocus = {
  field: 'stage' | 'application_date' | 'general'
}

const controlClass =
  'w-full rounded-lg border border-slate-300 bg-white px-3 py-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700 disabled:bg-slate-100'

const buttonClass =
  'rounded-lg border border-slate-300 bg-white px-4 py-2 font-medium hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700 disabled:opacity-60'

export default function PostingTracking(props: TrackingProps) {
  const [attempt, setAttempt] = useState(0)

  return (
    <section
      aria-labelledby="posting-tracking-heading"
      className="space-y-4 rounded-lg border border-slate-200 bg-white p-5"
    >
      <h2
        id="posting-tracking-heading"
        className="text-xl font-semibold"
      >
        Application tracking
      </h2>

      <TrackingLoader
        key={`${props.id}:${attempt}`}
        {...props}
        onRetry={() => setAttempt((current) => current + 1)}
      />
    </section>
  )
}

function TrackingLoader({
  id,
  disabled,
  onSavingChange,
  onMissing,
  onRetry,
}: TrackingProps & { onRetry: () => void }) {
  const [state, setState] = useState<TrackingLoadState>({
    status: 'loading',
  })

  useEffect(() => {
    const controller = new AbortController()
    void getPostingTracking(id, controller.signal)
      .then((tracking) => {
        if (!controller.signal.aborted) {
          setState({ status: 'success', tracking })
        }
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return
        }

        if (
          error instanceof PostingAPIError &&
          error.status === 404 &&
          error.code === 'not_found'
        ) {
          onMissing()
          return
        }

        setState({ status: 'error' })
      })

    return () => controller.abort()
  }, [id, onMissing])

  if (state.status === 'loading') {
    return <p role="status">Loading tracking...</p>
  }

  if (state.status === 'error') {
    return (
      <div className="space-y-3">
        <p role="alert" className="text-red-700">
          Unable to load tracking. Try again.
        </p>

        <button
          type="button"
          disabled={disabled}
          onClick={onRetry}
          className={buttonClass}
        >
          Retry tracking
        </button>
      </div>
    )
  }

  return (
    <TrackingEditor
      id={id}
      initial={state.tracking}
      disabled={disabled}
      onSavingChange={onSavingChange}
      onMissing={onMissing}
    />
  )
}

function TrackingEditor({
  id,
  initial,
  disabled,
  onSavingChange,
  onMissing,
}: TrackingProps & { initial: PostingTrackingData }) {
  const [saved, setSaved] = useState(initial)
  const [draft, setDraft] = useState<TrackingDraft>(() => ({
    stage: initial.stage,
    date: initial.application_date ?? '',
  }))
  const [dateInvalid, setDateInvalid] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [fieldErrors, setFieldErrors] =
    useState<PostingFieldErrors>({})
  const [message, setMessage] = useState('')
  const [success, setSuccess] = useState('')
  const [focusTarget, setFocusTarget] =
    useState<TrackingFocus | null>(null)

  const formRef = useRef<HTMLFormElement>(null)
  const feedbackRef = useRef<HTMLDivElement>(null)
  const activeRequest = useRef<AbortController | null>(null)

  const hasChanges =
    draft.stage !== saved.stage ||
    draft.date !== (saved.application_date ?? '') ||
    dateInvalid

  useEffect(() => {
    return () => activeRequest.current?.abort()
  }, [])

  useEffect(() => {
    if (!focusTarget) {
      return
    }

    if (focusTarget.field === 'general') {
      feedbackRef.current?.focus()
      return
    }

    const control =
      formRef.current?.elements.namedItem(focusTarget.field)

    if (control instanceof HTMLElement) {
      control.focus()
    }
  }, [focusTarget])

  function clearFeedback(field: 'stage' | 'application_date') {
    setMessage('')
    setSuccess('')
    setFieldErrors((current) => {
      const next = { ...current }
      delete next[field]
      return next
    })
  }

  function showError(
    nextMessage: string,
    fields: PostingFieldErrors = {},
  ) {
    setMessage(nextMessage)
    setFieldErrors(fields)
    setSuccess('')
    setFocusTarget({
      field: fields.stage
        ? 'stage'
        : fields.application_date
          ? 'application_date'
          : 'general',
    })
  }

  function resetChanges() {
    if (disabled || activeRequest.current) {
      return
    }

    formRef.current?.reset()
    setDraft({
      stage: saved.stage,
      date: saved.application_date ?? '',
    })
    setDateInvalid(false)
    setFieldErrors({})
    setMessage('')
    setSuccess('')
    setFocusTarget(null)
  }

  async function handleSubmit(
    event: SubmitEvent<HTMLFormElement>,
  ) {
    event.preventDefault()

    if (disabled || activeRequest.current || !hasChanges) {
      return
    }

    const dateControl =
      formRef.current?.elements.namedItem('application_date')

    if (
      dateControl instanceof HTMLInputElement &&
      !dateControl.validity.valid
    ) {
      showError('Check the highlighted field.', {
        application_date:
          'Enter a complete, valid date or clear it.',
      })
      return
    }

    const patch: PostingTrackingPatch = {}

    if (draft.stage !== saved.stage) {
      patch.stage = draft.stage
    }

    if (draft.date !== (saved.application_date ?? '')) {
      patch.application_date = draft.date || null
    }

    if (Object.keys(patch).length === 0) {
      return
    }

    const controller = new AbortController()
    activeRequest.current = controller
    setIsSaving(true)
    onSavingChange(true)
    setFieldErrors({})
    setMessage('')
    setSuccess('')

    try {
      const confirmed = await patchPostingTracking(
        id,
        patch,
        controller.signal,
      )

      if (!controller.signal.aborted) {
        setSaved(confirmed)
        setDraft({
          stage: confirmed.stage,
          date: confirmed.application_date ?? '',
        })
        setDateInvalid(false)
        setSuccess('Tracking saved.')
      }
    } catch (error: unknown) {
      if (controller.signal.aborted) {
        return
      }

      if (error instanceof PostingAPIError) {
        if (
          error.status === 404 &&
          error.code === 'not_found'
        ) {
          onMissing()
          return
        }

        if (
          error.status === 400 &&
          error.code === 'validation_failed'
        ) {
          showError(
            'Check the highlighted fields.',
            error.fieldErrors ?? {},
          )
          return
        }
      }

      showError(
        'Could not confirm the save. Check your connection and try Save tracking again.',
      )
    } finally {
      if (activeRequest.current === controller) {
        activeRequest.current = null
      }

      if (!controller.signal.aborted) {
        setIsSaving(false)
      }

      onSavingChange(false)
    }
  }

  return (
    <div className="space-y-5">
      <p className="text-sm text-slate-600">
        Saved stage: <strong>{postingStageLabel(saved.stage)}</strong>
        {' · '}Application date:{' '}
        <strong>{formatApplicationDate(saved.application_date)}</strong>
      </p>

      <form
        ref={formRef}
        noValidate
        onSubmit={handleSubmit}
        aria-busy={isSaving}
        className="space-y-4"
      >
        {message && (
          <div
            ref={feedbackRef}
            role="alert"
            tabIndex={-1}
            className="rounded-lg border border-red-300 bg-red-50 p-4 text-red-800 focus-visible:outline-2 focus-visible:outline-red-700"
          >
            {message}
          </div>
        )}

        <p role="status" className="min-h-6 text-slate-600">
          {isSaving ? 'Saving tracking…' : success}
        </p>

        <fieldset
          disabled={disabled || isSaving}
          className="min-w-0 space-y-4"
        >
          <legend className="sr-only">
            Tracking values and actions
          </legend>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label
                htmlFor="tracking-stage"
                className="block font-medium"
              >
                Stage
              </label>

              <select
                id="tracking-stage"
                name="stage"
                value={draft.stage}
                aria-invalid={Boolean(fieldErrors.stage)}
                aria-describedby={
                  fieldErrors.stage
                    ? 'tracking-stage-hint tracking-stage-error'
                    : 'tracking-stage-hint'
                }
                className={controlClass}
                onChange={(event) => {
                  const stage = event.target.value

                  if (isPostingStage(stage)) {
                    setDraft((current) => ({
                      ...current,
                      stage,
                    }))
                    clearFeedback('stage')
                  }
                }}
              >
                {postingStages.map((stage) => (
                  <option key={stage} value={stage}>
                    {postingStageLabel(stage)}
                  </option>
                ))}
              </select>

              <p
                id="tracking-stage-hint"
                className="text-sm text-slate-600"
              >
                Closed includes skipped or withdrawn opportunities.
              </p>

              {fieldErrors.stage && (
                <p
                  id="tracking-stage-error"
                  className="text-sm text-red-700"
                >
                  {fieldErrors.stage}
                </p>
              )}
            </div>

            <div className="space-y-2">
              <label
                htmlFor="tracking-application-date"
                className="block font-medium"
              >
                Application date (optional)
              </label>

              <input
                id="tracking-application-date"
                name="application_date"
                type="date"
                value={draft.date}
                aria-invalid={Boolean(fieldErrors.application_date)}
                aria-describedby={
                  fieldErrors.application_date
                    ? 'tracking-date-hint tracking-date-error'
                    : 'tracking-date-hint'
                }
                className={controlClass}
                onChange={(event) => {
                  const date = event.target.value
                  setDateInvalid(!event.target.validity.valid)
                  setDraft((current) => ({ ...current, date }))
                  clearFeedback('application_date')
                }}
              />

              <p
                id="tracking-date-hint"
                className="text-sm text-slate-600"
              >
                Clear to remove the date. Stage changes retain it.
              </p>

              {fieldErrors.application_date && (
                <p
                  id="tracking-date-error"
                  className="text-sm text-red-700"
                >
                  {fieldErrors.application_date}
                </p>
              )}
            </div>
          </div>

          <p className="text-sm text-slate-600">
            Changes take effect when you save.
          </p>

          <div className="flex flex-wrap gap-3">
            <button
              type="submit"
              disabled={!hasChanges}
              className="rounded-lg bg-slate-900 px-4 py-2 font-medium text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900 disabled:opacity-60"
            >
              {isSaving ? 'Saving…' : 'Save tracking'}
            </button>

            <button
              type="button"
              disabled={!hasChanges}
              onClick={resetChanges}
              className={buttonClass}
            >
              Reset changes
            </button>
          </div>
        </fieldset>
      </form>

      <TrackingHistory events={saved.history} />
    </div>
  )
}

function TrackingHistory({
  events,
}: {
  events: PostingStageEvent[]
}) {
  return (
    <section
      aria-labelledby="tracking-history-heading"
      className="space-y-3"
    >
      <h3
        id="tracking-history-heading"
        className="text-lg font-semibold"
      >
        Stage history
      </h3>

      {events.length === 0 ? (
        <p className="text-slate-600">
          No stage history has been saved. This posting appears
          as Inbox until you save tracking.
        </p>
      ) : (
        <ol className="space-y-3">
          {[...events].reverse().map((event) => (
            <li key={event.id} className="space-y-1">
              <p>
                {event.from_stage === null
                  ? `Started tracking as ${postingStageLabel(event.to_stage)}`
                  : `${postingStageLabel(event.from_stage)} → ${postingStageLabel(event.to_stage)}`}
              </p>

              <p className="text-sm text-slate-600">
                <time dateTime={event.changed_at}>
                  {formatPostingDate(event.changed_at)}
                </time>
              </p>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}