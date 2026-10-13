import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  getPosting,
  PostingAPIError,
  updatePosting,
} from '../api/postings'
import type { Posting, PostingInput } from '../api/postings'
import PostingForm from '../components/PostingForm'

type EditPostingState =
  | { status: 'loading' }
  | { status: 'success'; posting: Posting }
  | { status: 'invalid' }
  | { status: 'missing' }
  | { status: 'error' }

function PostingEditor({
  id,
  onRetry,
}: {
  id: string | undefined
  onRetry: () => void
}) {
  const [state, setState] = useState<EditPostingState>(() =>
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
          if (
            error.status === 400 &&
            error.code === 'invalid_id'
          ) {
            setState({ status: 'invalid' })
            return
          }

          if (
            error.status === 404 &&
            error.code === 'not_found'
          ) {
            setState({ status: 'missing' })
            return
          }
        }

        setState({ status: 'error' })
      })

    return () => controller.abort()
  }, [id])

  if (state.status === 'success') {
    return <PostingEditingForm posting={state.posting} />
  }

  const message = {
    loading: 'Loading posting…',
    invalid: 'This posting link is invalid.',
    missing: 'This posting was not found. It may have been deleted.',
    error: 'Unable to load this posting. Try again.',
  }[state.status]

  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-semibold">Edit Posting</h1>

      <p
        role={state.status === 'error' ? 'alert' : 'status'}
        className={
          state.status === 'error'
            ? 'text-red-700'
            : 'text-slate-600'
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

      <Link
        to="/postings"
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Back to postings
      </Link>
    </div>
  )
}

function PostingEditingForm({
  posting,
}: {
  posting: Posting
}) {
  const navigate = useNavigate()
  const [isMissing, setIsMissing] = useState(false)

  const initialValues: PostingInput = {
    company: posting.company,
    title: posting.title,
    url: posting.url,
    description: posting.description,
    location: posting.location,
    employment_type: posting.employment_type,
    workplace_arrangement: posting.workplace_arrangement,
  }

  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-semibold">Edit Posting</h1>

      <p className="wrap-anywhere text-lg text-slate-600">
        {posting.company} - {posting.title}
      </p>

      <p className="text-slate-600">
        Update the posting information.
      </p>

      <PostingForm
        initialValues={initialValues}
        onSave={(input, signal) =>
          updatePosting(String(posting.id), input, signal)
        }
        onSaved={(saved) =>
          navigate(`/postings/${saved.id}`, {
            replace: true,
          })
        }
        onMissing={() => setIsMissing(true)}
        onCancel={() =>
          navigate(
            isMissing ? '/postings' : `/postings/${posting.id}`,
            { replace: true },
          )
        }
      />
    </div>
  )
}

export default function EditPostingPage() {
  const { id } = useParams<{ id: string }>()
  const [attempt, setAttempt] = useState(0)

  return (
    <section className="space-y-6">
      <PostingEditor
        key={`${id ?? ''}:${attempt}`}
        id={id}
        onRetry={() => setAttempt((current) => current + 1)}
      />
    </section>
  )
}
