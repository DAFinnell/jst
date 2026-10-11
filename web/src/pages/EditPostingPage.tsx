import { Link, useParams } from 'react-router'

export default function EditPostingPage() {
  const { id } = useParams<{ id: string }>()

  return (
    <section className="space-y-4">
      <h1 className="text-3xl font-semibold">Edit Posting</h1>

      <p className="text-slate-600">Posting ID: {id}</p>
      <p className="text-slate-600">
        Edit form will go here.
      </p>

      <Link
        to={`/postings/${id}`}
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Back to posting
      </Link>
    </section>
  )
}
