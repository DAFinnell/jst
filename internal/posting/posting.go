package posting

import "strings"

const (
	EmploymentFullTime = "full_time"
	EmploymentOther    = "other"

	WorkplaceRemote = "remote"
	WorkplaceHybrid = "hybrid"
	WorkplaceOnSite = "on_site"
)

type Input struct {
	Company              string
	Title                string
	URL                  string
	Description          string
	Location             *string
	EmploymentType       *string
	WorkplaceArrangement *string
}

type ValidatedInput struct {
	Input
	NormalizedURL string
}

type Posting struct {
	ValidatedInput
	ID        int64
	Source    string
	CreatedAt string
	UpdatedAt string
}

type Summary struct {
	ID                   int64
	Company              string
	Title                string
	Location             *string
	EmploymentType       *string
	WorkplaceArrangement *string
}

const FilterUnknown = "unknown"

type ListOptions struct {
	Query                string
	EmploymentType       string
	WorkplaceArrangement string
}
type ValidationErrors map[string]string

func (e ValidationErrors) Error() string {
	return "Posting contains invalid fields."
}

func Validate(input Input) (ValidatedInput, error) {
	clean := input
	clean.Company = strings.TrimSpace(input.Company)
	clean.Title = strings.TrimSpace(input.Title)
	clean.URL = strings.TrimSpace(input.URL)
	clean.Location = cleanLocation(input.Location)

	fieldErrors := ValidationErrors{}

	if clean.Company == "" {
		fieldErrors["company"] = "Enter a company."
	}

	if clean.Title == "" {
		fieldErrors["title"] = "Enter a job title."
	}

	if strings.TrimSpace(clean.Description) == "" {
		fieldErrors["description"] = "Enter a job description."
	}

	var normalizedURL string
	if clean.URL == "" {
		fieldErrors["url"] = "Enter a URL."
	} else {
		var err error
		normalizedURL, err = NormalizeURL(clean.URL)
		if err != nil {
			fieldErrors["url"] = err.Error()
		}
	}

	if clean.EmploymentType != nil {
		switch *clean.EmploymentType {
		case EmploymentFullTime, EmploymentOther:
		default:
			fieldErrors["employment_type"] = "Choose Full-time, Other, or Unknown."
		}
	}

	if clean.WorkplaceArrangement != nil {
		switch *clean.WorkplaceArrangement {
		case WorkplaceRemote, WorkplaceHybrid, WorkplaceOnSite:
		default:
			fieldErrors["workplace_arrangement"] = "Choose Remote, Hybrid, On-site or Unknown."
		}
	}

	if len(fieldErrors) > 0 {
		return ValidatedInput{}, fieldErrors
	}

	return ValidatedInput{
		Input:         clean,
		NormalizedURL: normalizedURL,
	}, nil
}

func cleanLocation(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}
