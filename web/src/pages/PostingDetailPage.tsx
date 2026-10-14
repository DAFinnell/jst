import { useEffect, useRef, useState } from 'react'
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams
} from 'react-router'
import {
  deletePosting,
  getPosting,
  PostingAPIError,
} from '../api/postings'
import type { Posting } from '../api/postings'
import {
  employmentTypeLabel,
  formatPostingDate,
  workplaceArrangementLabel,
} from '../postingDisplay'
import {
  postingQuerySuffix,
  readPostingQuery,
} from '../postingQuery'

type PostingDetailState =
  | { status: 'loading' }
  | { status: 'success'; posting: Posting }
  | { status: 'invalid' }
  | { status: 'missing' }
  | { status: 'error' }

type DeletionState =
  | 'idle'
  | 'confirming'
  | 'deleting'
  | 'error'

type DeletionFocusTarget =
  | 'cancel'
  | 'delete'
  | 'feedback'
  | null

function PostingDetails({
  id,
  querySuffix,
  onRetry,
}: {
  id: string | undefined
  querySuffix: string
  onRetry: () => void
}) {
  const [state, setState] = useState<PostingDetailState>(() =>
    id ? { status: 'loading' } : { status: 'invalid' },
  )

  useEffect(() => {
    if (!id) {
      return
    }

    const controller = new AbortController()

    void getPosting(id, controller.signal)
      .then((posting) => {
        if (!controller.signal.aborted) {
          setState({ status: 'success', posting })
        }
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return
        }

        if (error instanceof PostingAPIError) {
          if (error.status === 400 && error.code === 'invalid_id') {
            setState({ status: 'invalid' })
            return
          }

          if (error.status === 404 && error.code === 'not_found') {
            setState({ status: 'missing' })
            return
          }
        }

        setState({ status: 'error' })
      })

    return () => controller.abort()
  }, [id])

  if (state.status === 'success') {
    return (
      <PostingInformation
        posting={state.posting}
        querySuffix={querySuffix}
        onMissing={() => setState({ status: 'missing' })}
        onReload={onRetry}
      />
    )
  }

  const message = {
    loading: 'Loading posting…',
    invalid: 'This posting link is invalid.',
    missing: 'This posting was not found. It may have been deleted.',
    error: 'Unable to load this posting. Try again.',
  }[state.status]

  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-semibold">Posting details</h1>

      <p
        role={state.status === 'error' ? 'alert' : 'status'}
        className={
          state.status === 'error' ? 'text-red-700' : 'text-slate-600'
        }
      >
        {message}
      </p>

      {state.status === 'error' && (
        <button
          type="button"
          onClick={onRetry}
          className="rounded-lg bg-slate-900 px-4 py-2 font-medium text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
        >
          Retry
        </button>
      )}
    </div>
  )
}

function PostingInformation({
  posting,
  querySuffix,
  onMissing,
  onReload,
}: {
  posting: Posting
  querySuffix: string
  onMissing: () => void
  onReload: () => void
}) {
  const navigate = useNavigate()
  const [deletion, setDeletion] =
    useState<DeletionState>('idle')
  const [focusTarget, setFocusTarget] =
    useState<DeletionFocusTarget>(null)

  const deleteButtonRef = useRef<HTMLButtonElement>(null)
  const cancelButtonRef = useRef<HTMLButtonElement>(null)
  const feedbackRef = useRef<HTMLDivElement>(null)
  const activeRequest = useRef<AbortController | null>(null)

  useEffect(() => {
    return () => activeRequest.current?.abort()
  }, [])

  useEffect(() => {
    if (focusTarget === 'cancel') {
      cancelButtonRef.current?.focus()
    } else if (focusTarget === 'delete') {
      deleteButtonRef.current?.focus()
    } else if (focusTarget === 'feedback') {
      feedbackRef.current?.focus()
    }
  }, [focusTarget])


  function openConfirmation() {
    setDeletion('confirming')
    setFocusTarget('cancel')
  }

  function cancelConfirmation() {
    setDeletion('idle')
    setFocusTarget('delete')
  }

  async function handleDelete() {
    if (deletion !== 'confirming' || activeRequest.current) {
      return
    }

    const controller = new AbortController()
    activeRequest.current = controller
    setDeletion('deleting')
    setFocusTarget('feedback')

    try {
      await deletePosting(String(posting.id), controller.signal)

      if (!controller.signal.aborted) {
        navigate(`/postings${querySuffix}`, {
          replace: true
        })
      }
    } catch (error: unknown) {
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

      setDeletion('error')
      setFocusTarget('feedback')
    } finally {
      if (activeRequest.current === controller) {
        activeRequest.current = null
      }
    }
  }

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <h1 className="wrap-anywhere text-3xl font-semibold">
          {posting.title}
        </h1>

        <p className="wrap-anywhere text-lg text-slate-600">
          {posting.company}
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-4">
        {deletion === 'deleting' ? (
          <span className="font-medium text-slate-500">
            Edit posting
          </span>
        ) : (
          <Link
            to={`/postings/${posting.id}/edit${querySuffix}`}
            className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
          >
            Edit posting
          </Link>
        )}

        <button
          ref={deleteButtonRef}
          type="button"
          onClick={openConfirmation}
          disabled={deletion !== 'idle'}
          aria-expanded={deletion !== 'idle'}
          aria-controls={
            deletion === 'idle'
              ? undefined
              : 'posting-delete-confirmation'
          }
          className="rounded-lg border border-red-300 bg-white px-4 py-2 font-medium text-red-700 hover:bg-red-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700 disabled:opacity-60"
        >
          Delete posting
        </button>
      </div>

      {deletion !== 'idle' && (
        <section
          id="posting-delete-confirmation"
          aria-labelledby="posting-delete-heading"
          className="space-y-4 rounded-lg border border-red-300 bg-red-50 p-5"
        >
          <h2
            id="posting-delete-heading"
            className="text-lg font-semibold"
          >
            Delete posting?
          </h2>

          <p className="wrap-anywhere">
            Permanently delete <strong>{posting.title}</strong>
            {' '}at <strong>{posting.company}</strong>?
            This cannot be undone.
          </p>

          {(deletion === 'deleting' || deletion === 'error') && (
            <div
              ref={feedbackRef}
              role={deletion === 'error' ? 'alert' : 'status'}
              tabIndex={-1}
              className="text-red-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700"
            >
              {deletion === 'deleting'
                ? 'Deleting posting…'
                : 'Could not confirm deletion. Reload this posting to check whether it still exists before trying again.'}
            </div>
          )}

          {deletion === 'error' ? (
            <button
              type="button"
              onClick={onReload}
              className="rounded-lg border border-slate-300 bg-white px-4 py-2 font-medium hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
            >
              Reload posting
            </button>
          ) : (
            <div className="flex flex-wrap gap-3">
              <button
                ref={cancelButtonRef}
                type="button"
                onClick={cancelConfirmation}
                disabled={deletion === 'deleting'}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 font-medium hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900 disabled:opacity-60"
              >
                Cancel
              </button>

              <button
                type="button"
                onClick={() => {
                  void handleDelete()
                }}
                disabled={deletion === 'deleting'}
                className="rounded-lg bg-red-700 px-4 py-2 font-medium text-white hover:bg-red-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700 disabled:opacity-60"
              >
                {deletion === 'deleting'
                  ? 'Deleting…'
                  : 'Delete permanently'}
              </button>
            </div>
          )}
        </section>
      )}

      <dl className="grid gap-4 rounded-lg border border-slate-200 bg-white p-5 sm:grid-cols-2">
        <div>
          <dt className="text-sm font-medium text-slate-600">Location</dt>
          <dd className="mt-1 wrap-anywhere">
            {posting.location ?? 'Unknown'}
          </dd>
        </div>

        <div>
          <dt className="text-sm font-medium text-slate-600">
            Employment type
          </dt>
          <dd className="mt-1">
            {employmentTypeLabel(posting.employment_type)}
          </dd>
        </div>

        <div>
          <dt className="text-sm font-medium text-slate-600">
            Workplace arrangement
          </dt>
          <dd className="mt-1">
            {workplaceArrangementLabel(posting.workplace_arrangement)}
          </dd>
        </div>

        <div>
          <dt className="text-sm font-medium text-slate-600">Source</dt>
          <dd className="mt-1">Manual</dd>
        </div>

        <div>
          <dt className="text-sm font-medium text-slate-600">Captured</dt>
          <dd className="mt-1">
            <time dateTime={posting.created_at}>
              {formatPostingDate(posting.created_at)}
            </time>
          </dd>
        </div>

        <div>
          <dt className="text-sm font-medium text-slate-600">
            Last updated
          </dt>
          <dd className="mt-1">
            <time dateTime={posting.updated_at}>
              {formatPostingDate(posting.updated_at)}
            </time>
          </dd>
        </div>

        <div className="sm:col-span-2">
          <dt className="text-sm font-medium text-slate-600">
            Original posting
          </dt>
          <dd className="mt-1 wrap-anywhere">
            <a
              href={posting.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
            >
              {posting.url}
              <span className="ml-2 text-sm">(opens in a new tab)</span>
            </a>
          </dd>
        </div>
      </dl>

      <section aria-labelledby="posting-description-heading" className="space-y-3">
        <h2
          id="posting-description-heading"
          className="text-xl font-semibold"
        >
          Description
        </h2>

        <div className="wrap-anywhere whitespace-pre-wrap rounded-lg border border-slate-200 bg-white p-5 leading-relaxed">
          {posting.description}
        </div>
      </section>
    </div>
  )
}

export default function PostingDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [params] = useSearchParams()
  const querySuffix = postingQuerySuffix(
    readPostingQuery(params),
  )
  const [attempt, setAttempt] = useState(0)

  return (
    <section className="space-y-6">
      <PostingDetails
        key={`${id ?? ''}:${attempt}`}
        id={id}
        querySuffix={querySuffix}
        onRetry={() => setAttempt((current) => current + 1)}
      />

      <Link
        to={`/postings${querySuffix}`}
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Back to postings
      </Link>
    </section>
  )
}
