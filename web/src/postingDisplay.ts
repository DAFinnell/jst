import type {
  EmploymentType,
  PostingStage,
  WorkplaceArrangement,
} from './api/postings'

const employmentLabels: Record<EmploymentType, string> = {
  full_time: 'Full-time',
  other: 'Other',
}

const workplaceLabels: Record<WorkplaceArrangement, string> = {
  remote: 'Remote',
  hybrid: 'Hybrid',
  on_site: 'On-site',
}

const dateFormatter = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
  timeZoneName: 'short',
})

export function employmentTypeLabel(
  value: EmploymentType | null,
): string {
  return value === null ? 'Unknown' : employmentLabels[value]
}

export function workplaceArrangementLabel(
  value: WorkplaceArrangement | null,
): string {
  return value === null ? 'Unknown' : workplaceLabels[value]
}

export function formatPostingDate(value: string): string {
  return dateFormatter.format(new Date(value))
}

const stageLabels: Record<PostingStage, string> = {
  inbox: 'Inbox',
  applied: 'Applied',
  interviewing: 'Interviewing',
  offer: 'Offer',
  rejected: 'Rejected',
  closed: 'Closed',
}

const applicationDateFormatter = new Intl.DateTimeFormat(
  undefined,
  {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  },
)

export function postingStageLabel(
  stage: PostingStage,
): string {
  return stageLabels[stage]
}

export function localToday(): string {
  const today = new Date()
  const year = String(today.getFullYear()).padStart(4, '0')
  const month = String(today.getMonth() + 1).padStart(2, '0')
  const day = String(today.getDate()).padStart(2, '0')

  return `${year}-${month}-${day}`
}

export function formatApplicationDate(
  value: string | null,
): string {
  if (value === null) {
    return 'Not set'
  }

  return applicationDateFormatter.format(
    new Date(`${value}T00:00:00Z`),
  )
}
