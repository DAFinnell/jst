import { useEffect, useState } from 'react'
import {
  Link,
  useLocation,
  useNavigationType,
  useSearchParams,
} from 'react-router'
import { getPostings } from '../api/postings'
import type { PostingSummary } from '../api/postings'
import {
  employmentTypeLabel,
  workplaceArrangementLabel,
} from '../postingDisplay'
import {
  postingQueryParams,
  postingQuerySuffix,
  readPostingQuery,
} from '../postingQuery'
import type { PostingQuery } from '../postingQuery'

const queryControlClass =
  'w-full min-w-0 rounded-lg border border-slate-300 bg-white px-3 py-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700'

type PostingListState =
  | { status: 'loading' }
  | { status: 'success'; items: PostingSummary[] }
  | { status: 'error' }

function PostingList({
  querySuffix,
  onRetry
}: {
  querySuffix: string
  onRetry: () => void
}) {
  const [state, setState] = useState<PostingListState>({
    status: 'loading',
  })

  useEffect(() => {
    const controller = new AbortController()

    const query = readPostingQuery(
      new URLSearchParams(querySuffix),
    )

    void getPostings(controller.signal, query)
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
  }, [querySuffix])

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
        {querySuffix
          ? 'No postings match your search and filters.'
          : 'No postings have been added yet.'}
      </p>
    )
  }

  return (
    <PostingsTable
      items={state.items}
      querySuffix={querySuffix}
    />
  )
}

function PostingsTable({
  items,
  querySuffix,
}: {
  items: PostingSummary[]
  querySuffix: string
}) {
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
                  to={`/postings/${posting.id}${querySuffix}`}
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
  const [params, setParams] = useSearchParams()
  const location = useLocation()
  const navigationType = useNavigationType()
  const query = readPostingQuery(params)
  const querySuffix = postingQuerySuffix(query)
  const [attempt, setAttempt] = useState(0)

  const [draft, setDraft] = useState(() => ({
    locationKey: location.key,
    appliedSearch: query.q,
    text: query.q,
  }))

  if (draft.locationKey !== location.key) {
    const restoreSearch =
      navigationType === 'POP' ||
      draft.appliedSearch !== query.q

    setDraft({
      locationKey: location.key,
      appliedSearch: query.q,
      text: restoreSearch ? query.q : draft.text,
    })
  }

  function applyQuery(
    next: PostingQuery,
    keepDraft = false,
  ) {
    if (!keepDraft) {
      setDraft({
        ...draft,
        text: next.q.trim(),
      })
    }

    if (postingQuerySuffix(next) !== querySuffix) {
      setParams(postingQueryParams(next))
    }
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-3xl font-semibold">Postings</h1>

        <Link
          to={`/postings/new${querySuffix}`}
          className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
        >
          Add posting
        </Link>
      </div>

      <form
        onSubmit={(event) => {
          event.preventDefault()
          applyQuery({ ...query, q: draft.text })
        }}
        className="grid items-end gap-3 sm:grid-cols-2 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_auto]"
      >
        <div className="flex min-w-0 items-end gap-2">
          <label className="flex min-w-0 flex-1 flex-col gap-1">
            <span className="text-sm font-medium">Search</span>
            <input
              type="search"
              name="q"
              value={draft.text}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  text: event.target.value,
                })
              }
              className={queryControlClass}
            />
          </label>

          <button
            type="submit"
            className="shrink-0 rounded-lg bg-slate-900 px-4 py-2 font-medium text-white hover:bg-slate-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
          >
            Search
          </button>
        </div>

        <label className="flex min-w-0 flex-col gap-1">
          <span className="text-sm font-medium">
            Employment type
          </span>
          <select
            name="employment_type"
            value={query.employment_type}
            onChange={(event) =>
              applyQuery(
                {
                  ...query,
                  employment_type: event.target
                    .value as PostingQuery['employment_type'],
                },
                true,
              )
            }
            className={queryControlClass}
          >
            <option value="">All</option>
            <option value="full_time">Full-time</option>
            <option value="unknown">Unknown</option>
            <option value="other">Other</option>
          </select>
        </label>

        <label className="flex min-w-0 flex-col gap-1">
          <span className="text-sm font-medium">
            Workplace arrangement
          </span>
          <select
            name="workplace_arrangement"
            value={query.workplace_arrangement}
            onChange={(event) =>
              applyQuery(
                {
                  ...query,
                  workplace_arrangement: event.target
                    .value as PostingQuery['workplace_arrangement'],
                },
                true,
              )
            }
            className={queryControlClass}
          >
            <option value="">All</option>
            <option value="remote">Remote</option>
            <option value="hybrid">Hybrid</option>
            <option value="on_site">On-site</option>
            <option value="unknown">Unknown</option>
          </select>
        </label>

        <button
          type="button"
          onClick={() =>
            applyQuery({
              q: '',
              employment_type: '',
              workplace_arrangement: '',
            })
          }
          className="justify-self-start rounded-lg border border-slate-300 bg-white px-4 py-2 font-medium hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-900"
        >
          Clear
        </button>
      </form>

      <PostingList
        key={`${querySuffix}:${attempt}`}
        querySuffix={querySuffix}
        onRetry={() => setAttempt((current) => current + 1)}
      />
    </section>
  )
}
