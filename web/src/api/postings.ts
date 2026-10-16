import { postingQuerySuffix } from '../postingQuery'
import type { PostingQuery } from '../postingQuery'

export type EmploymentType = 'full_time' | 'other'

export type WorkplaceArrangement = 'remote' | 'hybrid' | 'on_site'

export const postingStages = [
  'inbox',
  'applied',
  'interviewing',
  'offer',
  'rejected',
  'closed',
] as const

export type PostingStage = (typeof postingStages)[number]

export function isPostingStage(
  value: string | null,
): value is PostingStage {
  return postingStages.some((stage) => stage === value)
}

type PostingFields = {
  id: number
  company: string
  title: string
  location: string | null
  employment_type: EmploymentType | null
  workplace_arrangement: WorkplaceArrangement | null
}

export type PostingSummary = PostingFields & {
  stage: PostingStage
}

export type Posting = PostingFields & {
  url: string
  description: string
  source: 'manual'
  created_at: string
  updated_at: string
}

export type PostingInput = Pick<
  Posting,
  | 'company'
  | 'title'
  | 'url'
  | 'description'
  | 'location'
  | 'employment_type'
  | 'workplace_arrangement'
>

export type PostingCaptureInput = PostingInput & {
  initial_stage: PostingStage
  application_date: string | null
}

export type PostingStageEvent = {
  id: number
  from_stage: PostingStage | null
  to_stage: PostingStage
  changed_at: string
}

export type PostingTrackingData = {
  posting_id: number
  stage: PostingStage
  application_date: string | null
  created_at: string | null
  updated_at: string | null
  history: PostingStageEvent[]
}

export type PostingTrackingPatch = {
  stage?: PostingStage
  application_date?: string | null
}

export type PostingFieldErrors = Partial<
  Record<
    | keyof PostingInput
    | 'initial_stage'
    | 'stage'
    | 'application_date',
    string
  >
>

type PostingErrorResponse = {
  error?: {
    code?: string
    field_errors?: PostingFieldErrors
    existing_posting_id?: number
  }
}

type PostingListResponse = {
  items: PostingSummary[]
}

export class PostingAPIError extends Error {
  readonly status: number
  readonly code: string | undefined
  readonly fieldErrors: PostingFieldErrors | undefined
  readonly existingPostingId: number | undefined

  constructor(
    status: number,
    code?: string,
    fieldErrors?: PostingFieldErrors,
    existingPostingId?: number,
  ) {
    super(`Posting request failed (${status}).`)
    this.name = 'PostingAPIError'
    this.status = status
    this.code = code
    this.fieldErrors = fieldErrors
    this.existingPostingId = existingPostingId
  }
}

async function requestPosting(
  path: string,
  signal: AbortSignal,
  options: RequestInit = {}
): Promise<Response> {
  const response = await fetch(path, {
    ...options,
    signal: AbortSignal.any([
      signal,
      AbortSignal.timeout(5000)
    ]),
  })

  if (!response.ok) {
    let details: PostingErrorResponse['error']

    try {
      const body =
        (await response.json()) as PostingErrorResponse

      details = body.error
    } catch {
      // Keep HTTP status even if body isn't JSON.
    }

    throw new PostingAPIError(
      response.status,
      details?.code,
      details?.field_errors,
      details?.existing_posting_id,
    )
  }

  return response
}

async function requestPostingJSON<T>(
  path: string,
  signal: AbortSignal,
  options: RequestInit = {},
): Promise<T> {
  const response = await requestPosting(path, signal, options)

  return response.json() as Promise<T>
}

export async function getPostings(
  signal: AbortSignal,
  query: PostingQuery,
): Promise<PostingSummary[]> {
  const response =
    await requestPostingJSON<PostingListResponse>(
      `/api/v1/postings${postingQuerySuffix(query)}`,
      signal,
    )

  return response.items
}

export function getPosting(
  id: string,
  signal: AbortSignal,
): Promise<Posting> {
  return requestPostingJSON<Posting>(
    `/api/v1/postings/${encodeURIComponent(id)}`,
    signal,
  )
}

export function createPosting(
  input: PostingCaptureInput,
  signal: AbortSignal,
): Promise<Posting> {
  return requestPostingJSON<Posting>(
    '/api/v1/postings',
    signal,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export function updatePosting(
  id: string,
  input: PostingInput,
  signal: AbortSignal,
): Promise<Posting> {
  return requestPostingJSON<Posting>(
    `/api/v1/postings/${encodeURIComponent(id)}`,
    signal,
    {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}

export async function deletePosting(
  id: string,
  signal: AbortSignal,
): Promise<void> {
  await requestPosting(
    `/api/v1/postings/${encodeURIComponent(id)}`,
    signal,
    { method: 'DELETE' }
  )
}

export function getPostingTracking(
  id: string,
  signal: AbortSignal,
): Promise<PostingTrackingData> {
  return requestPostingJSON<PostingTrackingData>(
    `/api/v1/postings/${encodeURIComponent(id)}/tracking`,
    signal,
  )
}

export function patchPostingTracking(
  id: string,
  input: PostingTrackingPatch,
  signal: AbortSignal,
): Promise<PostingTrackingData> {
  return requestPostingJSON<PostingTrackingData>(
    `/api/v1/postings/${encodeURIComponent(id)}/tracking`,
    signal,
    {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    },
  )
}