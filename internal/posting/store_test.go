package posting

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/DAFinnell/jst/internal/storage"
)

func openPostingStore(t *testing.T) (*Store, *sql.DB) {
	t.Helper()

	db, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return NewStore(db), db
}

func createTestPosting(t *testing.T, store *Store, input Input) Posting {
	t.Helper()

	result, err := store.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func requirePostingIDs(
	t *testing.T,
	store *Store,
	options ListOptions,
	want ...int64,
) {
	t.Helper()

	items, err := store.List(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if items == nil {
		t.Fatal("successful listing returned a nil slice")
	}

	got := make([]int64, 0, len(items))
	for _, item := range items {
		got = append(got, item.ID)
	}

	if !slices.Equal(got, want) {
		t.Fatalf("posting IDs = %v, want %v", got, want)
	}
}

func requireDuplicate(t *testing.T, err error, wantID int64) {
	t.Helper()

	var duplicate *DuplicateURLError
	if !errors.As(err, &duplicate) {
		t.Fatalf("error = %v, want DuplicateURLError", err)
	}
	if duplicate.ExistingPostingID != wantID {
		t.Fatalf(
			"duplicate ID = %d, want %d",
			duplicate.ExistingPostingID,
			wantID,
		)
	}
}

func stampPosting(
	t *testing.T,
	db *sql.DB,
	id int64,
	createdAt string,
	updatedAt string,
) {
	t.Helper()

	if _, err := db.Exec(`
		UPDATE postings
		SET created_at = ?, updated_at = ?
		WHERE id = ?
	`, createdAt, updatedAt, id); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCreateGetAndReopen(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db, err := storage.Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	store := NewStore(db)
	description := "\n  <p>Build useful software.</p>\n**Go and React**  \n"

	input := Input{
		Company:              "  Example Company  ",
		Title:                "\tJunior Developer\n",
		URL:                  " HTTPS://Example.COM/jobs/123?utm_source=email#apply ",
		Description:          description,
		Location:             stringPointer("  Boston, MA  "),
		EmploymentType:       stringPointer(EmploymentFullTime),
		WorkplaceArrangement: stringPointer(WorkplaceRemote),
	}

	created, err := store.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	wantInput := Input{
		Company:              "Example Company",
		Title:                "Junior Developer",
		URL:                  "HTTPS://Example.COM/jobs/123?utm_source=email#apply",
		Description:          description,
		Location:             stringPointer("Boston, MA"),
		EmploymentType:       stringPointer(EmploymentFullTime),
		WorkplaceArrangement: stringPointer(WorkplaceRemote),
	}

	if !reflect.DeepEqual(created.Input, wantInput) {
		t.Fatalf("saved input = %#v, want %#v", created.Input, wantInput)
	}
	if created.NormalizedURL != "https://example.com/jobs/123" {
		t.Fatalf("normalized URL = %q", created.NormalizedURL)
	}
	if created.ID <= 0 || created.Source != "manual" {
		t.Fatalf("unexpected saved identity/source: %#v", created)
	}
	if created.CreatedAt != created.UpdatedAt {
		t.Fatal("initial creation and update timestamps differ")
	}
	if _, err := time.Parse(time.RFC3339Nano, created.CreatedAt); err != nil {
		t.Fatalf("invalid creation timestamp: %v", err)
	}

	fetched, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fetched, created) {
		t.Fatal("retrieved posting differs from the saved posting")
	}

	items, err := store.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	wantItems := []Summary{{
		ID:                   created.ID,
		Company:              "Example Company",
		Title:                "Junior Developer",
		Location:             stringPointer("Boston, MA"),
		EmploymentType:       stringPointer(EmploymentFullTime),
		WorkplaceArrangement: stringPointer(WorkplaceRemote),
	}}

	if !reflect.DeepEqual(items, wantItems) {
		t.Fatalf("summaries = %#v, want %#v", items, wantItems)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := storage.Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		reopened.Close()
	})

	persisted, err := NewStore(reopened).Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted, created) {
		t.Fatal("posting changed after reopening the database")
	}
}

func TestStoreOptionalValues(t *testing.T) {
	store, _ := openPostingStore(t)

	cases := []struct {
		name         string
		location     *string
		wantLocation *string
		employment   *string
		workplace    *string
	}{
		{"unknown", nil, nil, nil, nil},
		{"blank-location", stringPointer(" \t\n"), nil, nil, nil},
		{
			"full-time-remote",
			stringPointer("  Boston, MA  "),
			stringPointer("Boston, MA"),
			stringPointer(EmploymentFullTime),
			stringPointer(WorkplaceRemote),
		},
		{
			"other-hybrid", nil, nil,
			stringPointer(EmploymentOther),
			stringPointer(WorkplaceHybrid),
		},
		{
			"on-site", nil, nil, nil,
			stringPointer(WorkplaceOnSite),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			input.URL += "/" + test.name
			input.Location = test.location
			input.EmploymentType = test.employment
			input.WorkplaceArrangement = test.workplace

			created := createTestPosting(t, store, input)
			got, err := store.Get(context.Background(), created.ID)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, created) {
				t.Fatal("optional values changed between create and get")
			}
			if !reflect.DeepEqual(got.Location, test.wantLocation) ||
				!reflect.DeepEqual(got.EmploymentType, test.employment) ||
				!reflect.DeepEqual(got.WorkplaceArrangement, test.workplace) {
				t.Fatalf("unexpected optional values: %#v", got)
			}
		})
	}
}

func TestStoreUpdate(t *testing.T) {
	ctx := context.Background()
	store, db := openPostingStore(t)

	input := validInput()
	input.Location = stringPointer("Boston, MA")
	input.EmploymentType = stringPointer(EmploymentFullTime)
	input.WorkplaceArrangement = stringPointer(WorkplaceRemote)

	created := createTestPosting(t, store, input)

	const old = "2000-01-01T00:00:00.000Z"
	stampPosting(t, db, created.ID, old, old)

	if _, err := db.Exec(
		"UPDATE postings SET source = ? WHERE id = ?",
		"fixture_source", created.ID,
	); err != nil {
		t.Fatal(err)
	}

	original, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	edited := Input{
		Company:     "  Changed Company  ",
		Title:       "  Software Engineer  ",
		URL:         " HTTPS://Example.COM/jobs/456?utm_source=test#apply ",
		Description: "\n  Changed description.\n",
	}

	updated, err := store.Update(ctx, original.ID, edited)
	if err != nil {
		t.Fatal(err)
	}

	wantInput := Input{
		Company:     "Changed Company",
		Title:       "Software Engineer",
		URL:         "HTTPS://Example.COM/jobs/456?utm_source=test#apply",
		Description: "\n  Changed description.\n",
	}

	if !reflect.DeepEqual(updated.Input, wantInput) {
		t.Fatalf("updated input = %#v, want %#v", updated.Input, wantInput)
	}
	if updated.NormalizedURL != "https://example.com/jobs/456" {
		t.Fatalf("updated normalized URL = %q", updated.NormalizedURL)
	}
	if updated.ID != original.ID ||
		updated.Source != original.Source ||
		updated.CreatedAt != original.CreatedAt {
		t.Fatal("update changed ID, source, or creation time")
	}
	if updated.UpdatedAt == old {
		t.Fatal("update did not refresh modification time")
	}
	if _, err := time.Parse(time.RFC3339Nano, updated.UpdatedAt); err != nil {
		t.Fatalf("invalid update timestamp: %v", err)
	}

	persisted, err := store.Get(ctx, updated.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persisted, updated) {
		t.Fatal("update result differs from the stored record")
	}

	stampPosting(t, db, updated.ID, old, old)

	unchanged, err := store.Update(ctx, updated.ID, updated.Input)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.UpdatedAt == old {
		t.Fatal("unchanged save did not refresh modification time")
	}
	if !reflect.DeepEqual(unchanged.Input, updated.Input) ||
		unchanged.Source != updated.Source ||
		unchanged.CreatedAt != old {
		t.Fatal("unchanged save altered posting fields")
	}
}

func TestStoreInvalidWritesLeaveDataUnchanged(t *testing.T) {
	ctx := context.Background()
	store, _ := openPostingStore(t)
	original := createTestPosting(t, store, validInput())

	invalid := Input{
		EmploymentType:       stringPointer("part_time"),
		WorkplaceArrangement: stringPointer("office"),
	}

	operations := []struct {
		name string
		run  func() (Posting, error)
	}{
		{"create", func() (Posting, error) {
			return store.Create(ctx, invalid)
		}},
		{"update", func() (Posting, error) {
			return store.Update(ctx, original.ID, invalid)
		}},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			result, err := operation.run()

			var fieldErrors ValidationErrors
			if !errors.As(err, &fieldErrors) {
				t.Fatalf("error = %v, want ValidationErrors", err)
			}

			fields := []string{
				"company", "title", "url", "description",
				"employment_type", "workplace_arrangement",
			}
			if len(fieldErrors) != len(fields) {
				t.Fatalf("field errors = %#v, want all six fields", fieldErrors)
			}
			for _, field := range fields {
				if fieldErrors[field] == "" {
					t.Fatalf("missing error for %q", field)
				}
			}
			if result != (Posting{}) {
				t.Fatal("invalid save returned a usable posting")
			}

			requirePostingIDs(t, store, ListOptions{}, original.ID)

			persisted, err := store.Get(ctx, original.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(persisted, original) {
				t.Fatal("invalid save changed the original posting")
			}
		})
	}
}

func TestStoreDuplicateURLs(t *testing.T) {
	ctx := context.Background()
	store, _ := openPostingStore(t)

	first := createTestPosting(t, store, validInput())

	secondInput := validInput()
	secondInput.URL = "https://example.com/jobs/456"
	second := createTestPosting(t, store, secondInput)

	conflicting := validInput()
	conflicting.URL = " HTTPS://Example.COM/jobs/123?utm_source=test#apply "
	conflicting.Title = "Should not replace either posting"

	result, err := store.Create(ctx, conflicting)
	requireDuplicate(t, err, first.ID)
	if result != (Posting{}) {
		t.Fatal("conflicting create returned a posting")
	}

	result, err = store.Update(ctx, second.ID, conflicting)
	requireDuplicate(t, err, first.ID)
	if result != (Posting{}) {
		t.Fatal("conflicting update returned a posting")
	}

	for _, original := range []Posting{first, second} {
		persisted, err := store.Get(ctx, original.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(persisted, original) {
			t.Fatal("duplicate conflict changed an existing posting")
		}
	}

	ownURL := first.Input
	ownURL.URL = conflicting.URL

	updated, err := store.Update(ctx, first.ID, ownURL)
	if err != nil {
		t.Fatalf("posting conflicted with itself: %v", err)
	}
	if updated.ID != first.ID ||
		updated.NormalizedURL != first.NormalizedURL {
		t.Fatal("self edit changed posting identity")
	}

	items, err := store.List(ctx, ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("posting count = %d, want 2", len(items))
	}
}

func TestStoreMissingRecordsAndDeletion(t *testing.T) {
	ctx := context.Background()
	store, _ := openPostingStore(t)

	target := createTestPosting(t, store, validInput())

	otherInput := validInput()
	otherInput.URL = "https://example.com/jobs/456"
	other := createTestPosting(t, store, otherInput)

	const missingID int64 = 999999

	if _, err := store.Get(ctx, missingID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get error = %v, want ErrNotFound", err)
	}

	// This URL already exists, but the missing target takes precedence.
	if _, err := store.Update(ctx, missingID, target.Input); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update error = %v, want ErrNotFound", err)
	}

	if err := store.Delete(ctx, missingID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete error = %v, want ErrNotFound", err)
	}

	if err := store.Delete(ctx, target.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted posting get error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(ctx, target.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeated delete error = %v, want ErrNotFound", err)
	}

	requirePostingIDs(t, store, ListOptions{}, other.ID)

	if err := store.Delete(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	requirePostingIDs(t, store, ListOptions{})
}

func TestStoreListingSearchAndFilters(t *testing.T) {
	ctx := context.Background()
	store, db := openPostingStore(t)

	requirePostingIDs(t, store, ListOptions{})

	a := createTestPosting(t, store, Input{
		Company:              "Northstar Labs",
		Title:                "Platform Developer",
		URL:                  "https://example.com/jobs/a",
		Description:          "Build Go APIs; 100% remote with role_code.",
		Location:             stringPointer("Boston, MA"),
		EmploymentType:       stringPointer(EmploymentFullTime),
		WorkplaceArrangement: stringPointer(WorkplaceRemote),
	})

	b := createTestPosting(t, store, Input{
		Company:              "Cedar Systems",
		Title:                "Product Designer",
		URL:                  "https://example.com/jobs/b",
		Description:          "Research interfaces.",
		EmploymentType:       stringPointer(EmploymentOther),
		WorkplaceArrangement: stringPointer(WorkplaceHybrid),
	})

	c := createTestPosting(t, store, Input{
		Company:              "Open River",
		Title:                "Data Analyst",
		URL:                  "https://example.com/jobs/c",
		Description:          "Use SQL and Go.",
		WorkplaceArrangement: stringPointer(WorkplaceOnSite),
	})

	d := createTestPosting(t, store, Input{
		Company:     "Harbor Group",
		Title:       "Support Specialist",
		URL:         "https://example.com/jobs/d",
		Description: "Handle customer requests.",
	})

	stampPosting(t, db, a.ID,
		"2000-01-01T00:00:00.000Z", "2000-01-01T00:00:00.000Z")
	stampPosting(t, db, b.ID,
		"2002-01-01T00:00:00.000Z", "2002-01-01T00:00:00.000Z")
	stampPosting(t, db, c.ID,
		"2001-01-01T00:00:00.000Z", "2001-01-01T00:00:00.000Z")
	stampPosting(t, db, d.ID,
		"2002-01-01T00:00:00.000Z", "2002-01-01T00:00:00.000Z")

	cases := []struct {
		name    string
		options ListOptions
		want    []int64
	}{
		{"all", ListOptions{}, []int64{d.ID, b.ID, c.ID, a.ID}},
		{"blank search", ListOptions{Query: " \t\n"}, []int64{d.ID, b.ID, c.ID, a.ID}},
		{"company", ListOptions{Query: "NORTHSTAR"}, []int64{a.ID}},
		{"title", ListOptions{Query: "  developer  "}, []int64{a.ID}},
		{"description", ListOptions{Query: "sql"}, []int64{c.ID}},
		{"multiple matches", ListOptions{Query: "GO"}, []int64{c.ID, a.ID}},
		{"whole phrase", ListOptions{Query: "Go APIs"}, []int64{a.ID}},
		{"literal percent", ListOptions{Query: "%"}, []int64{a.ID}},
		{"literal underscore", ListOptions{Query: "_"}, []int64{a.ID}},
		{"no matches", ListOptions{Query: "unfindable"}, nil},
		{"URL is not searched", ListOptions{Query: "example.com"}, nil},
		{"SQL-like text", ListOptions{Query: "' OR 1=1 --"}, nil},

		{"full-time", ListOptions{EmploymentType: EmploymentFullTime}, []int64{a.ID}},
		{"other", ListOptions{EmploymentType: EmploymentOther}, []int64{b.ID}},
		{"unknown employment", ListOptions{EmploymentType: FilterUnknown}, []int64{d.ID, c.ID}},

		{"remote", ListOptions{WorkplaceArrangement: WorkplaceRemote}, []int64{a.ID}},
		{"hybrid", ListOptions{WorkplaceArrangement: WorkplaceHybrid}, []int64{b.ID}},
		{"on-site", ListOptions{WorkplaceArrangement: WorkplaceOnSite}, []int64{c.ID}},
		{"unknown workplace", ListOptions{WorkplaceArrangement: FilterUnknown}, []int64{d.ID}},

		{
			"both unknown",
			ListOptions{
				EmploymentType:       FilterUnknown,
				WorkplaceArrangement: FilterUnknown,
			},
			[]int64{d.ID},
		},
		{
			"search with unknown employment",
			ListOptions{Query: "Go", EmploymentType: FilterUnknown},
			[]int64{c.ID},
		},
		{
			"all restrictions",
			ListOptions{
				Query:                "Go",
				EmploymentType:       EmploymentFullTime,
				WorkplaceArrangement: WorkplaceRemote,
			},
			[]int64{a.ID},
		},
		{
			"combined no matches",
			ListOptions{Query: "Go", WorkplaceArrangement: WorkplaceHybrid},
			nil,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			requirePostingIDs(t, store, test.options, test.want...)
		})
	}

	edited := a.Input
	edited.Title = "Senior Platform Developer"

	if _, err := store.Update(ctx, a.ID, edited); err != nil {
		t.Fatal(err)
	}

	requirePostingIDs(t, store, ListOptions{}, d.ID, b.ID, c.ID, a.ID)
}

func TestStoreRejectsInvalidFilters(t *testing.T) {
	store, _ := openPostingStore(t)

	cases := []struct {
		name    string
		options ListOptions
		fields  []string
	}{
		{
			"employment",
			ListOptions{EmploymentType: "part_time"},
			[]string{"employment_type"},
		},
		{
			"workplace",
			ListOptions{WorkplaceArrangement: "office"},
			[]string{"workplace_arrangement"},
		},
		{
			"both",
			ListOptions{
				EmploymentType:       "part_time",
				WorkplaceArrangement: "office",
			},
			[]string{"employment_type", "workplace_arrangement"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			items, err := store.List(context.Background(), test.options)

			var fieldErrors ValidationErrors
			if !errors.As(err, &fieldErrors) {
				t.Fatalf("error = %v, want ValidationErrors", err)
			}
			if len(fieldErrors) != len(test.fields) {
				t.Fatalf("field errors = %#v", fieldErrors)
			}
			for _, field := range test.fields {
				if fieldErrors[field] == "" {
					t.Fatalf("missing error for %q", field)
				}
			}
			if items != nil {
				t.Fatal("invalid filters returned usable results")
			}
		})
	}
}

func TestStoreClassifiesDatabaseUniqueConflict(t *testing.T) {
	ctx := context.Background()
	store, db := openPostingStore(t)
	original := createTestPosting(t, store, validInput())

	_, writeErr := db.Exec(`
		INSERT INTO postings (
			company, title, url, normalized_url, description
		)
		SELECT company, title, url, normalized_url, description
		FROM postings
		WHERE id = ?
	`, original.ID)
	if writeErr == nil {
		t.Fatal("database accepted a duplicate normalized URL")
	}

	err := store.classifyWriteError(
		ctx,
		writeErr,
		original.NormalizedURL,
		0,
	)
	requireDuplicate(t, err, original.ID)

	requirePostingIDs(t, store, ListOptions{}, original.ID)
}

func TestStoreDatabaseFailuresRemainDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	store, db := openPostingStore(t)
	original := createTestPosting(t, store, validInput())

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	operations := []struct {
		name string
		run  func() error
	}{
		{"create", func() error {
			_, err := store.Create(ctx, validInput())
			return err
		}},
		{"get", func() error {
			_, err := store.Get(ctx, original.ID)
			return err
		}},
		{"update", func() error {
			_, err := store.Update(ctx, original.ID, original.Input)
			return err
		}},
		{"delete", func() error {
			return store.Delete(ctx, original.ID)
		}},
		{"list", func() error {
			_, err := store.List(ctx, ListOptions{})
			return err
		}},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.run()
			if err == nil {
				t.Fatal("operation succeeded with a closed database")
			}
			if errors.Is(err, ErrNotFound) {
				t.Fatal("database failure was reported as a missing posting")
			}

			var duplicate *DuplicateURLError
			if errors.As(err, &duplicate) {
				t.Fatal("database failure was reported as a duplicate")
			}

			var fieldErrors ValidationErrors
			if errors.As(err, &fieldErrors) {
				t.Fatal("database failure was reported as invalid input")
			}
		})
	}
}
