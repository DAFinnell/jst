import { useNavigate } from 'react-router'
import { createPosting } from '../api/postings'
import PostingForm from '../components/PostingForm'

export default function NewPostingPage() {
  const navigate = useNavigate()

  return (
    <section className="space-y-4">
      <h1 className="text-3xl font-semibold">
        Add Posting
      </h1>

      <p className="text-slate-600">
        Input the info from a posting.
        Required fields are marked.
      </p>

      <PostingForm
        onSave={createPosting}
        onSaved={(posting) =>
          navigate(`/postings/${posting.id}`, {
            replace: true,
          })
        }
        onCancel={() => navigate('/postings')}
      />
    </section>
  )
}
