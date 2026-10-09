package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestPostingSchemaFreshDatabase(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM postings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("posting count = %d, want 0", count)
	}

	var strict int
	if err := db.QueryRow(
		"SELECT strict FROM pragma_table_list WHERE schema = 'main' AND name = 'postings'",
	).Scan(&strict); err != nil {
		t.Fatal(err)
	}
	if strict != 1 {
		t.Fatal("postings table is not STRICT")
	}

	if _, err := db.Exec(`
		INSERT INTO postings (
			company, title, url, normalized_url, description
		) VALUES (?, ?, ?, ?, ?)
	`,
		"Example Company",
		"Junior Developer",
		"https://example.com/jobs/1",
		"https://example.com/jobs/1",
		"First line\nSecond line",
	); err != nil {
		t.Fatal(err)
	}

	var id int64
	var description, source, createdAt, updatedAt string
	var location, employment, workplace sql.NullString

	if err := db.QueryRow(`
		SELECT id, description, location, employment_type,
		       workplace_arrangement, source, created_at, updated_at
		FROM postings
	`).Scan(
		&id, &description, &location, &employment,
		&workplace, &source, &createdAt, &updatedAt,
	); err != nil {
		t.Fatal(err)
	}

	if id <= 0 {
		t.Fatalf("posting ID = %d, want a positive ID", id)
	}
	if description != "First line\nSecond line" {
		t.Fatalf("description = %q, want preserved line breaks", description)
	}
	if location.Valid || employment.Valid || workplace.Valid {
		t.Fatal("omitted optional fields should be NULL")
	}
	if source != "manual" {
		t.Fatalf("source = %q, want manual", source)
	}
	if _, err := time.Parse(time.RFC3339Nano, createdAt); err != nil {
		t.Fatalf("invalid creation timestamp %q: %v", createdAt, err)
	}
	if updatedAt != createdAt {
		t.Fatalf("initial timestamps differ: %q and %q", createdAt, updatedAt)
	}
}

func TestPostingSchemaConstraints(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const insert = `
		INSERT INTO postings (
			company, title, url, normalized_url, description,
			employment_type, workplace_arrangement
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	validCases := []struct {
		url        string
		employment any
		workplace  any
	}{
		{"https://example.com/jobs/1", nil, nil},
		{"https://example.com/jobs/2", "full_time", "remote"},
		{"https://example.com/jobs/3", "other", "hybrid"},
		{"https://example.com/jobs/4", nil, "on_site"},
	}

	for _, test := range validCases {
		if _, err := db.Exec(
			insert,
			"Example Company", "Developer", test.url, test.url,
			"Example description", test.employment, test.workplace,
		); err != nil {
			t.Fatalf("valid posting %q was rejected: %v", test.url, err)
		}
	}

	invalidCases := []struct {
		name        string
		company     any
		title       any
		url         any
		normalized  any
		description any
		employment  any
		workplace   any
	}{
		{"missing company", nil, "Developer", "https://example.com/new", "https://example.com/new", "Description", nil, nil},
		{"missing title", "Example Company", nil, "https://example.com/new", "https://example.com/new", "Description", nil, nil},
		{"missing URL", "Example Company", "Developer", nil, "https://example.com/new", "Description", nil, nil},
		{"missing normalized URL", "Example Company", "Developer", "https://example.com/new", nil, "Description", nil, nil},
		{"missing description", "Example Company", "Developer", "https://example.com/new", "https://example.com/new", nil, nil, nil},
		{"duplicate normalized URL", "Example Company", "Developer", "https://example.com/jobs/1?utm_source=test", "https://example.com/jobs/1", "Description", nil, nil},
		{"invalid employment", "Example Company", "Developer", "https://example.com/new", "https://example.com/new", "Description", "part_time", nil},
		{"invalid workplace", "Example Company", "Developer", "https://example.com/new", "https://example.com/new", "Description", nil, "office"},
		{"unknown must be NULL", "Example Company", "Developer", "https://example.com/new", "https://example.com/new", "Description", "unknown", nil},
	}

	for _, test := range invalidCases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.Exec(
				insert,
				test.company, test.title, test.url, test.normalized,
				test.description, test.employment, test.workplace,
			); err == nil {
				t.Fatal("expected the database to reject this posting")
			}
		})
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM postings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(validCases) {
		t.Fatalf("posting count = %d, want %d", count, len(validCases))
	}
}

func TestPostingSchemaUpgradesM0Database(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	initSQL, err := migrationFiles.ReadFile("migrations/00001_init.sql")
	if err != nil {
		t.Fatal(err)
	}

	m0Source := fstest.MapFS{
		"00001_init.sql": &fstest.MapFile{Data: initSQL},
	}

	m0DB, err := sql.Open("sqlite", filepath.Join(dataDir, "jst.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer m0DB.Close()

	if err := migrate(ctx, m0DB, m0Source); err != nil {
		t.Fatal(err)
	}

	var originalCreatedAt string
	if err := m0DB.QueryRow(
		"SELECT value FROM app_metadata WHERE key = 'created_at'",
	).Scan(&originalCreatedAt); err != nil {
		t.Fatal(err)
	}

	if _, err := m0DB.Exec(
		"INSERT INTO app_metadata (key, value) VALUES (?, ?)",
		"checkpoint_1_upgrade_test", "preserved",
	); err != nil {
		t.Fatal(err)
	}

	var tableCount int
	if err := m0DB.QueryRow(
		"SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name = 'postings'",
	).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatal("M0 fixture unexpectedly contains postings")
	}

	if err := m0DB.Close(); err != nil {
		t.Fatal(err)
	}

	for _, phase := range []string{"upgrade", "reopen"} {
		db, err := Open(ctx, dataDir)
		if err != nil {
			t.Fatalf("%s: %v", phase, err)
		}
		defer db.Close()

		for key, want := range map[string]string{
			"created_at":                originalCreatedAt,
			"checkpoint_1_upgrade_test": "preserved",
		} {
			var got string
			if err := db.QueryRow(
				"SELECT value FROM app_metadata WHERE key = ?", key,
			).Scan(&got); err != nil {
				t.Fatalf("%s: read metadata %q: %v", phase, key, err)
			}
			if got != want {
				t.Fatalf("%s: metadata %q = %q, want %q", phase, key, got, want)
			}
		}

		var postingCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM postings").Scan(&postingCount); err != nil {
			t.Fatalf("%s: %v", phase, err)
		}
		if postingCount != 0 {
			t.Fatalf("%s: posting count = %d, want 0", phase, postingCount)
		}

		for _, version := range []int{1, 2} {
			var appliedCount int
			if err := db.QueryRow(`
				SELECT COUNT(*) FROM goose_db_version
				WHERE version_id = ? AND is_applied = 1
			`, version).Scan(&appliedCount); err != nil {
				t.Fatalf("%s: %v", phase, err)
			}
			if appliedCount != 1 {
				t.Fatalf(
					"%s: migration %d applied records = %d, want 1",
					phase, version, appliedCount,
				)
			}
		}

		if err := db.Close(); err != nil {
			t.Fatalf("%s: close database: %v", phase, err)
		}
	}
}
