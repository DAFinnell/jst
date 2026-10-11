import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { getPostings } from '../api/postings'
import type { PostingSummary } from '../api/postings'
import {
  employmentTypeLabel,
  workplaceArrangementLabel,
} from '../postingDisplay'

type PostingListState =
  | { status: 'loading' }
  | { status: 'success'; items: PostingSummary[] }
  | { status: 'error' }

function PostingList({ onRetry }: { onRetry: () => void }) {
  const [state, setState] = useState<PostingListState>({
    status: 'loading',
  })

  useEffect(() => {
    const controller = new AbortController()

    void getPostings(controller.signal)
      .then((items) => {
        if (!controller.signal.aborted) {
          setState({ status: 'success', items })
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setState({ status: 'error' })
        }
      })

    return () => controller.abort()
  }, [])

  if (state.status === 'loading') {
    return <p role="status">Loading postings...</p>
  }

  if (state.status === 'error') {
    return (
      <div className="space-y-3">
        <p role="alert" className="text-red-700">
          Unable to load postings. Try again.
        </p>

        <button
          onClick={onRetry}
          className="rounded-lg bg-slate-900 px-4 py-2 font-medium text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
        >
          Retry
        </button>
      </div>
    )
  }

  if (state.items.length === 0) {
    return (
      <p role="status" className="text-slate-600">
        No postings saved yet.
      </p>
    )
  }

  return <PostingsTable items={state.items} />
}

function PostingsTable({ items }: { items: PostingSummary[] }) {
  return (
    <div
      role="region"
      aria-label="Saved postings table"
      tabIndex={0}
      className="max-w-full overflow-x-auto rounded-lg border border-slate-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
    >
      <table className="w-full min-w-[48rem] table-fixed bg-white text-left text-sm">
        <caption className="sr-only">Saved postings</caption>

        <thead className="bg-slate-100 text-slate-700">
          <tr>
            <th scope="col" className="w-1/5 px-4 py-3 font-semibold">
              Company
            </th>
            <th scope="col" className="w-[30%] px-4 py-3 font-semibold">
              Title
            </th>
            <th scope="col" className="w-1/5 px-4 py-3 font-semibold">
              Location
            </th>
            <th scope="col" className="w-[15%] px-4 py-3 font-semibold">
              Employment type
            </th>
            <th scope="col" className="w-[15%] px-4 py-3 font-semibold">
              Workplace arrangement
            </th>
          </tr>
        </thead>

        <tbody>
          {items.map((posting) => (
            <tr
              key={posting.id}
              className="border-t border-slate-200 align-top"
            >
              <td className="wrap-anywhere px-4 py-3">
                {posting.company}
              </td>

              <td className="wrap-anywhere px-4 py-3">
                <Link
                  to={`/postings/${posting.id}`}
                  className="font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
                >
                  {posting.title}
                </Link>
              </td>

              <td className="wrap-anywhere px-4 py-3">
                {posting.location ?? 'Unknown'}
              </td>

              <td className="wrap-anywhere px-4 py-3">
                {employmentTypeLabel(posting.employment_type)}
              </td>

              <td className="wrap-anywhere px-4 py-3">
                {workplaceArrangementLabel(posting.workplace_arrangement)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function PostingsPage() {
  const [attempt, setAttempt] = useState(0)

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-3xl font-semibold">Postings</h1>

        <Link
          to="/postings/new"
          className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
        >
          Add posting
        </Link>
      </div>

      <PostingList
        key={attempt}
        onRetry={() => setAttempt((current) => current + 1)}
      />
    </section>
  )
}
