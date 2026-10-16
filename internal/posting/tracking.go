package posting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	StageInbox        = "inbox"
	StageApplied      = "applied"
	StageInterviewing = "interviewing"
	StageOffer        = "offer"
	StageRejected     = "rejected"
	StageClosed       = "closed"
)

type StageEvent struct {
	ID        int64
	FromStage *string
	ToStage   string
	ChangedAt string
}

type Tracking struct {
	PostingID       int64
	Stage           string
	ApplicationDate *string
	CreatedAt       *string
	UpdatedAt       *string
	History         []StageEvent
}

type TrackingPatch struct {
	Stage              *string
	ApplicationDate    *string
	ApplicationDateSet bool
}

type CaptureInput struct {
	Input
	InitialStage    *string
	ApplicationDate *string
}

func validStage(stage string) bool {
	switch stage {
	case StageInbox, StageApplied, StageInterviewing,
		StageOffer, StageRejected, StageClosed:
		return true
	default:
		return false
	}
}

func trackingFieldErrors(patch TrackingPatch) ValidationErrors {
	fields := ValidationErrors{}

	if patch.Stage == nil && !patch.ApplicationDateSet {
		fields["stage"] = "Choose a stage or supply an application date."
	}

	if patch.Stage != nil && !validStage(*patch.Stage) {
		fields["stage"] =
			"Choose Inbox, Applied, Interviewing, Offer, Rejected, or Closed."
	}

	if patch.ApplicationDateSet && patch.ApplicationDate != nil {
		if _, err := time.Parse(time.DateOnly, *patch.ApplicationDate); err != nil {
			fields["application_date"] =
				"Enter a real date using YYYY-MM-DD, or clear the date."
		}
	}

	return fields
}

func sameDate(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func readTracking(
	ctx context.Context,
	tx *sql.Tx,
	id int64,
) (Tracking, error) {
	result := Tracking{
		History: make([]StageEvent, 0),
	}

	err := tx.QueryRowContext(ctx, `
		SELECT p.id, COALESCE(t.stage, 'inbox'),
			t.application_date, t.created_at, t.updated_at
		FROM postings AS p
		LEFT JOIN posting_tracking AS t ON t.posting_id = p.id
		WHERE p.id = ?
	`, id).Scan(
		&result.PostingID,
		&result.Stage,
		&result.ApplicationDate,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Tracking{}, ErrNotFound
	}
	if err != nil {
		return Tracking{}, fmt.Errorf("read tracking: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, from_stage, to_stage, changed_at
		FROM posting_stage_history
		WHERE posting_id = ?
		ORDER BY id
	`, id)
	if err != nil {
		return Tracking{}, fmt.Errorf("read stage history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var event StageEvent
		if err := rows.Scan(
			&event.ID,
			&event.FromStage,
			&event.ToStage,
			&event.ChangedAt,
		); err != nil {
			return Tracking{}, fmt.Errorf("scan stage history: %w", err)
		}
		result.History = append(result.History, event)
	}
	if err := rows.Err(); err != nil {
		return Tracking{}, fmt.Errorf("iterate stage history: %w", err)
	}

	return result, nil
}

func (s *Store) GetTracking(
	ctx context.Context,
	id int64,
) (Tracking, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tracking{}, fmt.Errorf("begin tracking read: %w", err)
	}
	defer tx.Rollback()

	result, err := readTracking(ctx, tx, id)
	if err != nil {
		return Tracking{}, err
	}

	if err := tx.Commit(); err != nil {
		return Tracking{}, fmt.Errorf("finish tracking read: %w", err)
	}
	return result, nil
}

func applyTracking(
	ctx context.Context,
	tx *sql.Tx,
	id int64,
	patch TrackingPatch,
) error {
	current, err := readTracking(ctx, tx, id)
	if err != nil {
		return err
	}

	stage := current.Stage
	if patch.Stage != nil {
		stage = *patch.Stage
	}

	date := current.ApplicationDate
	if patch.ApplicationDateSet {
		date = patch.ApplicationDate
	}

	initial := current.CreatedAt == nil
	stageChanged := stage != current.Stage

	if !initial && !stageChanged && sameDate(date, current.ApplicationDate) {
		return nil
	}

	if initial {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO posting_tracking
				(posting_id, stage, application_date
			) VALUES (?, ?, ?)
		`, id, stage, date)
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE posting_tracking
			SET stage = ?, application_date = ?,
				updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
			WHERE posting_id = ?
		`, stage, date, id)
	}
	if err != nil {
		return fmt.Errorf("save tracking card: %w", err)
	}

	if initial || stageChanged {
		var fromStage *string
		if !initial {
			fromStage = &current.Stage
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO posting_stage_history (
				posting_id, from_stage, to_stage
			) VALUES (?, ?, ?)
		`, id, fromStage, stage); err != nil {
			return fmt.Errorf("append stage history: %w", err)
		}
	}

	return nil
}

func (s *Store) SaveTracking(
	ctx context.Context,
	id int64,
	patch TrackingPatch,
) (Tracking, error) {
	if fields := trackingFieldErrors(patch); len(fields) > 0 {
		return Tracking{}, fields
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tracking{}, fmt.Errorf("begin tracking save: %w", err)
	}
	defer tx.Rollback()

	if err := applyTracking(ctx, tx, id, patch); err != nil {
		return Tracking{}, err
	}

	result, err := readTracking(ctx, tx, id)
	if err != nil {
		return Tracking{}, err
	}

	if err := tx.Commit(); err != nil {
		return Tracking{}, fmt.Errorf("commit tracking save: %w", err)
	}
	return result, nil
}

func (s *Store) Capture(
	ctx context.Context,
	input CaptureInput,
) (Posting, error) {
	stage := StageApplied
	if input.InitialStage != nil {
		stage = *input.InitialStage
	}

	patch := TrackingPatch{
		Stage:              &stage,
		ApplicationDate:    input.ApplicationDate,
		ApplicationDateSet: true,
	}

	fields := trackingFieldErrors(patch)
	if message, exists := fields["stage"]; exists {
		fields["initial_stage"] = message
		delete(fields, "stage")
	}

	validated, err := Validate(input.Input)
	if err != nil {
		var sourceFields ValidationErrors
		if !errors.As(err, &sourceFields) {
			return Posting{}, err
		}
		for field, message := range sourceFields {
			fields[field] = message
		}
	}
	if len(fields) > 0 {
		return Posting{}, fields
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Posting{}, fmt.Errorf("begin capture: %w", err)
	}
	defer tx.Rollback()

	saved, err := createPosting(ctx, tx, validated)
	if err != nil {
		return Posting{}, err
	}

	if err := applyTracking(ctx, tx, saved.ID, patch); err != nil {
		return Posting{}, err
	}

	if err := tx.Commit(); err != nil {
		return Posting{}, fmt.Errorf("commit capture: %w", err)
	}
	return saved, nil
}
