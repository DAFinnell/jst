import type {
  EmploymentType,
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