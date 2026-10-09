package storage

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestOpenPreservesDataAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dataDir := filepath.Join(t.TempDir(), "data")

	db, err := Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var createdAt string
	if err := db.QueryRow(
		"SELECT value FROM app_metadata WHERE key = 'created_at'",
	).Scan(&createdAt); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(
		"INSERT INTO app_metadata (key, value) VALUES (?, ?)",
		"persistence_test",
		"preserved",
	); err != nil {
		t.Fatal(err)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	var value string
	if err := reopened.QueryRow(
		"SELECT value FROM app_metadata WHERE key = 'persistence_test'",
	).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "preserved" {
		t.Fatalf("value = %q, want preserved", value)
	}

	var reopenedCreatedAt string
	if err := reopened.QueryRow(
		"SELECT value FROM app_metadata WHERE key = 'created_at'",
	).Scan(&reopenedCreatedAt); err != nil {
		t.Fatal(err)
	}
	if reopenedCreatedAt != createdAt {
		t.Fatal("database creation time changed after reopening")
	}

	var migrationCount int
	if err := reopened.QueryRow(
		"SELECT COUNT(*) FROM goose_db_version WHERE version_id = 1 AND is_applied = 1",
	).Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration 1 applied records = %d, want 1", migrationCount)
	}

	info, err := os.Stat(filepath.Join(dataDir, "attachments"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("attachments path is not a directory")
	}
}

func TestOpenRejectsInvalidDataDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}

	db, err := Open(context.Background(), path)
	if db != nil {
		db.Close()
		t.Fatal("unexpected database for invalid data directory")
	}
	if err == nil {
		t.Fatal("expected an initialization error")
	}
}

func TestFailedMigrationRollsBack(t *testing.T) {
	ctx := context.Background()

	db, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	source, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		t.Fatal(err)
	}

	brokenSource := fstest.MapFS{}
	for _, entry := range entries {
		content, err := fs.ReadFile(source, entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		brokenSource[entry.Name()] = &fstest.MapFile{Data: content}
	}

	brokenSource["99999_rollback_test.sql"] = &fstest.MapFile{
		Data: []byte(`
-- +goose Up
CREATE TABLE rollback_probe (id INTEGER);
INSERT INTO missing_test_table_99999 VALUES (1);

-- +goose Down
DROP TABLE rollback_probe;
`),
	}

	if err := migrate(ctx, db, brokenSource); err == nil {
		t.Fatal("expected migration failure")
	}

	var tableCount int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'rollback_probe'",
	).Scan(&tableCount); err != nil {
		t.Fatal(err)
	}
	if tableCount != 0 {
		t.Fatal("failed migration left a partial table behind")
	}

	var appliedCount int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM goose_db_version WHERE version_id = 99999 AND is_applied = 1",
	).Scan(&appliedCount); err != nil {
		t.Fatal(err)
	}
	if appliedCount != 0 {
		t.Fatal("failed migration was recorded as applied")
	}
}
