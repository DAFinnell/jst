import { useNavigate, useSearchParams } from 'react-router'
import { createPosting } from '../api/postings'
import PostingForm from '../components/PostingForm'
import {
  postingQuerySuffix,
  readPostingQuery,
} from '../postingQuery'

export default function NewPostingPage() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const querySuffix = postingQuerySuffix(
    readPostingQuery(params),
  )

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
        mode="capture"
        onSave={createPosting}
        onSaved={(posting) =>
          navigate(`/postings/${posting.id}${querySuffix}`, {
            replace: true,
          })
        }
        onCancel={() =>
          navigate(`/postings${querySuffix}`)
        }
      />
    </section>
  )
}
