import { Link } from 'react-router'

export default function PostingsPage() {
  return (
    <section className="space-y-4">
      <h1 className="text-3xl font-semibold">Postings</h1>

      <p className="text-slate-600">
        Saved postings will appear here.
      </p>

      <Link
        to="/postings/new"
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Add New Posting
      </Link>
    </section>
  )
}
