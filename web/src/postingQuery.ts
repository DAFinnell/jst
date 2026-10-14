import type {
  EmploymentType,
  WorkplaceArrangement,
} from './api/postings'

export type PostingQuery = {
  q: string
  employment_type: EmploymentType | 'unknown' | ''
  workplace_arrangement: WorkplaceArrangement | 'unknown' | ''
}

export function readPostingQuery(
  params: URLSearchParams,
): PostingQuery {
  const employment = params.get('employment_type')
  const workplace = params.get('workplace_arrangement')

  return {
    q: (params.get('q') ?? '').trim(),
    employment_type:
      employment === 'full_time' ||
        employment === 'other' ||
        employment === 'unknown'
        ? employment
        : '',
    workplace_arrangement:
      workplace === 'remote' ||
        workplace === 'hybrid' ||
        workplace === 'on_site' ||
        workplace === 'unknown'
        ? workplace
        : '',
  }
}

export function postingQueryParams(
  query: PostingQuery,
): URLSearchParams {
  const params = new URLSearchParams()

  const search = query.q.trim()

  if (search) {
    params.set('q', search)
  }

  if (query.employment_type) {
    params.set('employment_type', query.employment_type)
  }

  if (query.workplace_arrangement) {
    params.set(
      'workplace_arrangement',
      query.workplace_arrangement,
    )
  }

  return params
}

export function postingQuerySuffix(
  query: PostingQuery,
): string {
  const search = postingQueryParams(query).toString()

  return search ? `?${search}` : ''
}