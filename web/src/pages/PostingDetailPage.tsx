import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { getPosting, PostingAPIError } from '../api/postings'
import type { Posting } from '../api/postings'
import {
  employmentTypeLabel,
  formatPostingDate,
  workplaceArrangementLabel,
} from '../postingDisplay'

type PostingDetailState =
  | { status: 'loading' }
  | { status: 'success'; posting: Posting }
  | { status: 'invalid' }
  | { status: 'missing' }
  | { status: 'error' }

function PostingDetails({
  id,
  onRetry,
}: {
  id: string | undefined
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
    return <PostingInformation posting={state.posting} />
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

function PostingInformation({ posting }: { posting: Posting }) {
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

      <Link
        to={`/postings/${posting.id}/edit`}
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Edit posting
      </Link>

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
  const [attempt, setAttempt] = useState(0)

  return (
    <section className="space-y-6">
      <PostingDetails
        key={`${id ?? ''}:${attempt}`}
        id={id}
        onRetry={() => setAttempt((current) => current + 1)}
      />

      <Link
        to="/postings"
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Back to postings
      </Link>
    </section>
  )
}
