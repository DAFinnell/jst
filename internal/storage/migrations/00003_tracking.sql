-- +goose Up
CREATE TABLE posting_tracking (
    posting_id INTEGER PRIMARY KEY
        REFERENCES postings(id) ON DELETE CASCADE,
    stage TEXT NOT NULL
        CHECK (stage IN (
            'inbox', 'applied', 'interviewing',
            'offer', 'rejected', 'closed'
        )),
    application_date TEXT,
    created_at TEXT NOT NULL
        DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL
        DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE TABLE posting_stage_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    posting_id INTEGER NOT NULL
        REFERENCES posting_tracking(posting_id) ON DELETE CASCADE,
    from_stage TEXT
        CHECK (from_stage IN (
            'inbox', 'applied', 'interviewing',
            'offer', 'rejected', 'closed'
        )),
    to_stage TEXT NOT NULL
        CHECK (to_stage IN (
            'inbox', 'applied', 'interviewing',
            'offer', 'rejected', 'closed'
        )),
    changed_at TEXT NOT NULL
        DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX posting_stage_history_posting_id_id
    ON posting_stage_history (posting_id, id);

-- +goose Down
DROP TABLE posting_stage_history;
DROP TABLE posting_tracking;