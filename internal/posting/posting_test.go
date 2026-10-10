package posting

import (
	"errors"
	"reflect"
	"testing"
)

func stringPointer(value string) *string {
	return &value
}

func validInput() Input {
	return Input{
		Company:     "Example Company",
		Title:       "Junior Developer",
		URL:         "https://example.com/jobs/123",
		Description: "First line\nSecond line",
	}
}

func TestValidateCleansShortFieldsAndPreservesDescription(t *testing.T) {
	description := "\n  <p>First line</p>\n**Second line**  \n"

	input := validInput()
	input.Company = "  Example Company  "
	input.Title = "\tJunior Developer\n"
	input.URL = " HTTPS://Example.COM/jobs/123?utm_source=email#apply "
	input.Description = description
	input.Location = stringPointer("  Boston, MA  ")

	result, err := Validate(input)
	if err != nil {
		t.Fatal(err)
	}

	want := Input{
		Company:     "Example Company",
		Title:       "Junior Developer",
		URL:         "HTTPS://Example.COM/jobs/123?utm_source=email#apply",
		Description: description,
		Location:    stringPointer("Boston, MA"),
	}

	if !reflect.DeepEqual(result.Input, want) {
		t.Fatalf("validated input = %#v, want %#v", result.Input, want)
	}
	if result.NormalizedURL != "https://example.com/jobs/123" {
		t.Fatalf("normalized URL = %q", result.NormalizedURL)
	}

	if input.Company != "  Example Company  " ||
		*input.Location != "  Boston, MA  " {
		t.Fatal("validation changed the original input")
	}
}

func TestValidateOptionalValues(t *testing.T) {
	cases := []struct {
		name       string
		location   *string
		employment *string
		workplace  *string
	}{
		{"unknown values", nil, nil, nil},
		{"blank location", stringPointer(" \t\n"), nil, nil},
		{"full-time remote", nil,
			stringPointer(EmploymentFullTime), stringPointer(WorkplaceRemote)},
		{"other hybrid", nil,
			stringPointer(EmploymentOther), stringPointer(WorkplaceHybrid)},
		{"unknown on-site", nil, nil, stringPointer(WorkplaceOnSite)},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			input.Location = test.location
			input.EmploymentType = test.employment
			input.WorkplaceArrangement = test.workplace

			result, err := Validate(input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Location != nil {
				t.Fatal("missing or blank location should become nil")
			}
			if !reflect.DeepEqual(result.EmploymentType, test.employment) {
				t.Fatal("employment value changed")
			}
			if !reflect.DeepEqual(result.WorkplaceArrangement, test.workplace) {
				t.Fatal("workplace value changed")
			}
		})
	}
}

func TestValidateRejectsEmptyRequiredFields(t *testing.T) {
	fields := []struct {
		name string
		set  func(*Input, string)
	}{
		{"company", func(input *Input, value string) { input.Company = value }},
		{"title", func(input *Input, value string) { input.Title = value }},
		{"url", func(input *Input, value string) { input.URL = value }},
		{"description", func(input *Input, value string) { input.Description = value }},
	}

	for _, field := range fields {
		for _, value := range []string{"", " \t\n\u00a0"} {
			t.Run(field.name+"/"+value, func(t *testing.T) {
				input := validInput()
				field.set(&input, value)

				_, err := Validate(input)

				var fieldErrors ValidationErrors
				if !errors.As(err, &fieldErrors) {
					t.Fatalf("error = %v, want ValidationErrors", err)
				}
				if len(fieldErrors) != 1 || fieldErrors[field.name] == "" {
					t.Fatalf("field errors = %#v", fieldErrors)
				}
			})
		}
	}
}

func TestValidateCollectsAllFieldErrors(t *testing.T) {
	input := Input{
		Company:              " ",
		Title:                "\t",
		URL:                  "https://user:password@example.com/jobs/123",
		Description:          "\n",
		EmploymentType:       stringPointer("part_time"),
		WorkplaceArrangement: stringPointer("office"),
	}

	result, err := Validate(input)

	var fieldErrors ValidationErrors
	if !errors.As(err, &fieldErrors) {
		t.Fatalf("error = %v, want ValidationErrors", err)
	}

	want := ValidationErrors{
		"company":               "Enter a company.",
		"title":                 "Enter a job title.",
		"url":                   "URLs must not contain a username or password.",
		"description":           "Enter a job description.",
		"employment_type":       "Choose Full-time, Other, or Unknown.",
		"workplace_arrangement": "Choose Remote, Hybrid, On-site or Unknown.",
	}

	if !reflect.DeepEqual(fieldErrors, want) {
		t.Fatalf("field errors = %#v, want %#v", fieldErrors, want)
	}
	if result != (ValidatedInput{}) {
		t.Fatal("invalid input should not return a validated posting")
	}
}

func TestValidateRejectsUnsupportedCategories(t *testing.T) {
	for _, value := range []string{"unknown", "", "FULL_TIME"} {
		t.Run(value, func(t *testing.T) {
			input := validInput()
			input.EmploymentType = stringPointer(value)
			input.WorkplaceArrangement = stringPointer(value)

			_, err := Validate(input)

			var fieldErrors ValidationErrors
			if !errors.As(err, &fieldErrors) {
				t.Fatalf("error = %v, want ValidationErrors", err)
			}
			if len(fieldErrors) != 2 ||
				fieldErrors["employment_type"] == "" ||
				fieldErrors["workplace_arrangement"] == "" {
				t.Fatalf("field errors = %#v", fieldErrors)
			}
		})
	}
}

func TestValidateRejectsWorkplaceValueAsEmployment(t *testing.T) {
	input := validInput()
	input.EmploymentType = stringPointer(WorkplaceRemote)

	_, err := Validate(input)

	var fieldErrors ValidationErrors
	if !errors.As(err, &fieldErrors) {
		t.Fatalf("error = %v, want ValidationErrors", err)
	}
	if len(fieldErrors) != 1 || fieldErrors["employment_type"] == "" {
		t.Fatalf("field errors = %#v, want an employment error", fieldErrors)
	}
}
