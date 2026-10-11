import { Link } from 'react-router'

export default function NewPostingPage() {
  return (
    <section className="space-y-4">
      <h1 className="text-3xl font-semibold">Add Posting</h1>

      <p className="text-slate-600">
        Posting form will go here.
      </p>

      <Link
        to="/postings"
        className="inline-block font-medium text-blue-700 underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        Back to Postings
      </Link>
    </section>
  )
}
