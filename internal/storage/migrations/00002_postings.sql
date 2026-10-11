-- +goose Up
CREATE TABLE postings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    company TEXT NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    normalized_url TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    location TEXT,
    employment_type TEXT
        CHECK (employment_type IN ('full_time', 'other')),
    workplace_arrangement TEXT
        CHECK (workplace_arrangement IN ('remote', 'hybrid', 'on_site')),
    source TEXT NOT NULL DEFAULT 'manual',
    created_at TEXT NOT NULL
        DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL
        DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

-- +goose Down
DROP TABLE postings;