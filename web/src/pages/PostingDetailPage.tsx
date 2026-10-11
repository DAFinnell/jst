import { Link, useParams } from 'react-router'

export default function PostingDetailPage() {
  const { id } = useParams<{ id: string }>()

  return (
    <section className="space-y-4">
      <h1 className="text-3xl font-semibold">Posting details</h1>

      <p className="text-slate-600">Posting ID: {id}</p>
      <p className="text-slate-600">
        More posting details will go here.
      </p>

      <div className="flex flex-wrap gap-4">
        <Link
          to={`/postings/${id}/edit`}
          className="font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
        >
          Edit Posting
        </Link>

        <Link
          to="/postings"
          className="font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
        >
          Back to Postings
        </Link>
      </div>
    </section>
  )
}
