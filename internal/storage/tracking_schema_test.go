package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func execTrackingSQL(
	t *testing.T,
	db *sql.DB,
	query string,
	args ...any,
) {
	t.Helper()

	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("execute tracking fixture SQL: %v", err)
	}
}

func requireTrackingCount(
	t *testing.T,
	db *sql.DB,
	want int,
	query string,
	args ...any,
) {
	t.Helper()

	var got int
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s: got %d, want %d", query, got, want)
	}
}

func seedTrackingPosting(
	t *testing.T,
	db *sql.DB,
	url string,
) int64 {
	t.Helper()

	result, err := db.Exec(`
		INSERT INTO postings (
			company, title, url, normalized_url, description
		) VALUES (?, ?, ?, ?, ?)
	`,
		"Example Company",
		"Developer",
		url,
		url,
		"Example description",
	)
	if err != nil {
		t.Fatal(err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	return id
}

func TestTrackingSchemaFreshDatabaseAndConstraints(t *testing.T) {
	db, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	requireTrackingCount(t, db, 2, `
		SELECT COUNT(*)
		FROM pragma_table_list
		WHERE schema = 'main'
		  AND strict = 1
		  AND name IN ('posting_tracking', 'posting_stage_history')
	`)
	requireTrackingCount(t, db, 0,
		"SELECT COUNT(*) FROM posting_tracking")
	requireTrackingCount(t, db, 0,
		"SELECT COUNT(*) FROM posting_stage_history")

	stages := []string{
		"inbox", "applied", "interviewing",
		"offer", "rejected", "closed",
	}

	var firstID int64

	for i, stage := range stages {
		id := seedTrackingPosting(
			t, db, "https://example.com/jobs/"+stage,
		)
		if i == 0 {
			firstID = id
		}

		execTrackingSQL(t, db, `
			INSERT INTO posting_tracking (posting_id, stage)
			VALUES (?, ?)
		`, id, stage)

		execTrackingSQL(t, db, `
			INSERT INTO posting_stage_history (posting_id, to_stage)
			VALUES (?, ?)
		`, id, stage)
	}

	requireTrackingCount(t, db, len(stages), `
		SELECT COUNT(*)
		FROM posting_tracking
		WHERE application_date IS NULL
	`)
	requireTrackingCount(t, db, len(stages), `
		SELECT COUNT(*)
		FROM posting_stage_history
		WHERE from_stage IS NULL
	`)

	var createdAt, updatedAt, changedAt string

	if err := db.QueryRow(`
		SELECT created_at, updated_at
		FROM posting_tracking
		WHERE posting_id = ?
	`, firstID).Scan(&createdAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`
		SELECT changed_at
		FROM posting_stage_history
		WHERE posting_id = ?
	`, firstID).Scan(&changedAt); err != nil {
		t.Fatal(err)
	}

	if createdAt != updatedAt {
		t.Fatal("initial tracking timestamps differ")
	}

	for _, stamp := range []string{createdAt, updatedAt, changedAt} {
		parsed, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			t.Fatalf("invalid timestamp %q: %v", stamp, err)
		}
		if parsed.Location() != time.UTC {
			t.Fatalf("timestamp %q is not UTC", stamp)
		}
	}

	untrackedID := seedTrackingPosting(
		t, db, "https://example.com/jobs/untracked",
	)

	invalidWrites := []struct {
		name  string
		query string
		args  []any
	}{
		{
			"duplicate tracking card",
			`INSERT INTO posting_tracking (posting_id, stage)
			 VALUES (?, 'applied')`,
			[]any{firstID},
		},
		{
			"invalid tracking stage",
			`INSERT INTO posting_tracking (posting_id, stage)
			 VALUES (?, 'screening')`,
			[]any{untrackedID},
		},
		{
			"missing posting",
			`INSERT INTO posting_tracking (posting_id, stage)
			 VALUES (?, 'inbox')`,
			[]any{int64(999999)},
		},
		{
			"invalid history destination",
			`INSERT INTO posting_stage_history (posting_id, to_stage)
			 VALUES (?, 'screening')`,
			[]any{firstID},
		},
		{
			"invalid history previous stage",
			`INSERT INTO posting_stage_history (
				posting_id, from_stage, to_stage
			 ) VALUES (?, 'screening', 'applied')`,
			[]any{firstID},
		},
		{
			"history without tracking",
			`INSERT INTO posting_stage_history (posting_id, to_stage)
			 VALUES (?, 'inbox')`,
			[]any{untrackedID},
		},
	}

	for _, test := range invalidWrites {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.Exec(test.query, test.args...); err == nil {
				t.Fatal("expected the database to reject this write")
			}
		})
	}

	requireTrackingCount(t, db, len(stages),
		"SELECT COUNT(*) FROM posting_tracking")
	requireTrackingCount(t, db, len(stages),
		"SELECT COUNT(*) FROM posting_stage_history")
}

func TestTrackingSchemaPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	db, err := Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	appliedID := seedTrackingPosting(
		t, db, "https://example.com/jobs/applied",
	)
	inboxID := seedTrackingPosting(
		t, db, "https://example.com/jobs/inbox",
	)

	execTrackingSQL(t, db, `
		INSERT INTO posting_tracking (
			posting_id, stage, application_date
		) VALUES
			(?, 'applied', '2026-10-12'),
			(?, 'inbox', NULL)
	`, appliedID, inboxID)

	const eventTime = "2026-10-14T12:00:00.000Z"

	execTrackingSQL(t, db, `
		INSERT INTO posting_stage_history (
			posting_id, from_stage, to_stage, changed_at
		) VALUES
			(?, NULL, 'applied', ?),
			(?, 'applied', 'interviewing', ?),
			(?, 'interviewing', 'applied', ?)
	`,
		appliedID, eventTime,
		appliedID, eventTime,
		appliedID, eventTime,
	)

	execTrackingSQL(t, db, `
		INSERT INTO posting_stage_history (posting_id, to_stage)
		VALUES (?, 'inbox')
	`, inboxID)

	var originalCreatedAt, originalUpdatedAt string
	if err := db.QueryRow(`
		SELECT created_at, updated_at
		FROM posting_tracking
		WHERE posting_id = ?
	`, appliedID).Scan(
		&originalCreatedAt, &originalUpdatedAt,
	); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	expected := []struct {
		id    int64
		stage string
		date  sql.NullString
	}{
		{
			appliedID,
			"applied",
			sql.NullString{String: "2026-10-12", Valid: true},
		},
		{inboxID, "inbox", sql.NullString{}},
	}

	for _, test := range expected {
		var stage string
		var date sql.NullString

		if err := reopened.QueryRow(`
			SELECT stage, application_date
			FROM posting_tracking
			WHERE posting_id = ?
		`, test.id).Scan(&stage, &date); err != nil {
			t.Fatal(err)
		}

		if stage != test.stage || date != test.date {
			t.Fatalf(
				"posting %d: stage/date = %q/%#v, want %q/%#v",
				test.id, stage, date, test.stage, test.date,
			)
		}
	}

	var createdAt, updatedAt string
	if err := reopened.QueryRow(`
		SELECT created_at, updated_at
		FROM posting_tracking
		WHERE posting_id = ?
	`, appliedID).Scan(&createdAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if createdAt != originalCreatedAt || updatedAt != originalUpdatedAt {
		t.Fatal("reopening changed tracking timestamps")
	}

	requireTrackingCount(t, reopened, 1, `
		SELECT COUNT(*)
		FROM posting_stage_history
		WHERE posting_id = ?
		  AND from_stage IS NULL
		  AND to_stage = 'inbox'
	`, inboxID)

	rows, err := reopened.Query(`
		SELECT id, from_stage, to_stage, changed_at
		FROM posting_stage_history
		WHERE posting_id = ?
		ORDER BY id
	`, appliedID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	wantFrom := []sql.NullString{
		{},
		{String: "applied", Valid: true},
		{String: "interviewing", Valid: true},
	}
	wantTo := []string{"applied", "interviewing", "applied"}

	var previousID int64
	index := 0

	for rows.Next() {
		var id int64
		var fromStage sql.NullString
		var toStage, changedAt string

		if err := rows.Scan(
			&id, &fromStage, &toStage, &changedAt,
		); err != nil {
			t.Fatal(err)
		}

		if index >= len(wantTo) {
			t.Fatal("unexpected extra history entry")
		}
		if id <= previousID {
			t.Fatal("history IDs are not increasing")
		}
		if fromStage != wantFrom[index] ||
			toStage != wantTo[index] ||
			changedAt != eventTime {
			t.Fatalf(
				"history entry %d = %#v -> %q at %q",
				index, fromStage, toStage, changedAt,
			)
		}

		previousID = id
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(wantTo) {
		t.Fatalf("history count = %d, want %d", index, len(wantTo))
	}
}

func TestTrackingSchemaUpgradesM1Database(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()

	m1Source := fstest.MapFS{}

	for _, name := range []string{
		"00001_init.sql",
		"00002_postings.sql",
	} {
		content, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		m1Source[name] = &fstest.MapFile{Data: content}
	}

	m1DB, err := sql.Open(
		"sqlite", filepath.Join(dataDir, "jst.sqlite"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer m1DB.Close()

	if err := migrate(ctx, m1DB, m1Source); err != nil {
		t.Fatal(err)
	}

	requireTrackingCount(t, m1DB, 0, `
		SELECT COUNT(*)
		FROM sqlite_schema
		WHERE type = 'table'
		  AND name IN ('posting_tracking', 'posting_stage_history')
	`)

	var originalDatabaseCreatedAt string
	if err := m1DB.QueryRow(`
		SELECT value FROM app_metadata WHERE key = 'created_at'
	`).Scan(&originalDatabaseCreatedAt); err != nil {
		t.Fatal(err)
	}

	execTrackingSQL(t, m1DB, `
		INSERT INTO app_metadata (key, value)
		VALUES ('m2_checkpoint_1_upgrade_test', 'preserved')
	`)

	originalPosting := []any{
		int64(41),
		"M1 Example Company",
		"Developer",
		"https://example.com/jobs/legacy",
		"https://example.com/jobs/legacy",
		"First line\nSecond line",
		nil,
		"full_time",
		"remote",
		"manual",
		"2026-10-01T10:00:00.000Z",
		"2026-10-02T11:00:00.000Z",
	}

	execTrackingSQL(t, m1DB, `
		INSERT INTO postings (
			id, company, title, url, normalized_url, description,
			location, employment_type, workplace_arrangement,
			source, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, originalPosting...)

	if err := m1DB.Close(); err != nil {
		t.Fatal(err)
	}

	for _, phase := range []string{"upgrade", "reopen"} {
		t.Run(phase, func(t *testing.T) {
			db, err := Open(ctx, dataDir)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			requireTrackingCount(t, db, 1,
				"SELECT COUNT(*) FROM postings")

			requireTrackingCount(t, db, 1, `
				SELECT COUNT(*)
				FROM postings
				WHERE id IS ?
				  AND company IS ?
				  AND title IS ?
				  AND url IS ?
				  AND normalized_url IS ?
				  AND description IS ?
				  AND location IS ?
				  AND employment_type IS ?
				  AND workplace_arrangement IS ?
				  AND source IS ?
				  AND created_at IS ?
				  AND updated_at IS ?
			`, originalPosting...)

			for key, want := range map[string]string{
				"created_at":                   originalDatabaseCreatedAt,
				"m2_checkpoint_1_upgrade_test": "preserved",
			} {
				var got string
				if err := db.QueryRow(`
					SELECT value FROM app_metadata WHERE key = ?
				`, key).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf(
						"metadata %q = %q, want %q",
						key, got, want,
					)
				}
			}

			requireTrackingCount(t, db, 0,
				"SELECT COUNT(*) FROM posting_tracking")
			requireTrackingCount(t, db, 0,
				"SELECT COUNT(*) FROM posting_stage_history")

			for _, version := range []int{1, 2, 3} {
				requireTrackingCount(t, db, 1, `
					SELECT COUNT(*)
					FROM goose_db_version
					WHERE version_id = ? AND is_applied = 1
				`, version)
			}
		})
	}
}
