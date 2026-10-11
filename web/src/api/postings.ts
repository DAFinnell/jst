export type EmploymentType = 'full_time' | 'other'

export type WorkplaceArrangement = 'remote' | 'hybrid' | 'on_site'

export type PostingSummary = {
  id: number
  company: string
  title: string
  location: string | null
  employment_type: EmploymentType | null
  workplace_arrangement: WorkplaceArrangement | null
}

export type Posting = PostingSummary & {
  url: string
  description: string
  source: 'manual'
  created_at: string
  updated_at: string
}

type PostingListResponse = {
  items: PostingSummary[]
}

export class PostingAPIError extends Error {
  readonly status: number
  readonly code: string | undefined

  constructor(status: number, code?: string) {
    super(`Posting request failed (${status}).`)
    this.name = 'PostingAPIError'
    this.status = status
    this.code = code
  }
}

async function readPostingJSON<T>(
  path: string,
  signal: AbortSignal,
): Promise<T> {
  const response = await fetch(path, {
    signal: AbortSignal.any([
      signal,
      AbortSignal.timeout(5000)
    ]),
  })

  if (!response.ok) {
    let code: string | undefined

    try {
      const body = (await response.json()) as {
        error?: { code?: string }
      }

      code = body.error?.code
    } catch {
      // Keep HTTP status even if body isn't JSON.
    }

    throw new PostingAPIError(response.status, code)
  }

  return response.json() as Promise<T>
}

export async function getPostings(
  signal: AbortSignal,
): Promise<PostingSummary[]> {
  const response = await readPostingJSON<PostingListResponse>(
    '/api/v1/postings',
    signal,
  )

  return response.items
}

export function getPosting(
  id: string,
  signal: AbortSignal,
): Promise<Posting> {
  return readPostingJSON<Posting>(
    `/api/v1/postings/${encodeURIComponent(id)}`,
    signal,
  )
}
