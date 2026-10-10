package posting

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrNotFound = errors.New("posting was not found")

type DuplicateURLError struct {
	ExistingPostingID int64
}

func (e *DuplicateURLError) Error() string {
	return "A posting with this URL already exists."
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

const postingColumns = `
	id, company, title, url, normalized_url, description,
	location, employment_type, workplace_arrangement,
	source, created_at, updated_at
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPosting(row rowScanner) (Posting, error) {
	var result Posting
	err := row.Scan(
		&result.ID,
		&result.Company,
		&result.Title,
		&result.URL,
		&result.NormalizedURL,
		&result.Description,
		&result.Location,
		&result.EmploymentType,
		&result.WorkplaceArrangement,
		&result.Source,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if err != nil {
		return Posting{}, err
	}
	return result, nil
}

func (s *Store) checkDuplicate(
	ctx context.Context,
	normalizedURL string,
	excludeID int64,
) error {
	var existingID int64

	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM postings
		WHERE normalized_url = ? AND id <> ?
	`, normalizedURL, excludeID).Scan(&existingID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check posting URL: %w", err)
	}

	return &DuplicateURLError{ExistingPostingID: existingID}
}

func (s *Store) classifyWriteError(
	ctx context.Context,
	writeErr error,
	normalizedURL string,
	excludeId int64,
) error {
	var sqliteErr *sqlite.Error

	if errors.As(writeErr, &sqliteErr) &&
		sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		if err := s.checkDuplicate(ctx, normalizedURL, excludeId); err != nil {
			return err
		}
	}

	return writeErr
}

func (s *Store) Create(ctx context.Context, input Input) (Posting, error) {
	validated, err := Validate(input)
	if err != nil {
		return Posting{}, err
	}

	if err := s.checkDuplicate(ctx, validated.NormalizedURL, 0); err != nil {
		return Posting{}, fmt.Errorf("create posting: %w", err)
	}

	result, err := scanPosting(s.db.QueryRowContext(ctx, `
		INSERT INTO postings (
			company, title, url, normalized_url, description,
			location, employment_type, workplace_arrangement
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING `+postingColumns,
		validated.Company,
		validated.Title,
		validated.URL,
		validated.NormalizedURL,
		validated.Description,
		validated.Location,
		validated.EmploymentType,
		validated.WorkplaceArrangement,
	))
	if err != nil {
		return Posting{}, fmt.Errorf(
			"create posting: %w",
			s.classifyWriteError(ctx, err, validated.NormalizedURL, 0),
		)
	}

	return result, nil
}

func (s *Store) Get(ctx context.Context, id int64) (Posting, error) {
	result, err := scanPosting(s.db.QueryRowContext(ctx, `
		SELECT `+postingColumns+`
		FROM postings
		WHERE id = ?
	`, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Posting{}, ErrNotFound
	}
	if err != nil {
		return Posting{}, fmt.Errorf("get posting: %w", err)
	}

	return result, nil
}

func (s *Store) Update(
	ctx context.Context,
	id int64,
	input Input,
) (Posting, error) {
	validated, err := Validate(input)
	if err != nil {
		return Posting{}, err
	}

	if _, err := s.Get(ctx, id); err != nil {
		return Posting{}, fmt.Errorf("update posting: %w", err)
	}

	if err := s.checkDuplicate(ctx, validated.NormalizedURL, id); err != nil {
		return Posting{}, fmt.Errorf("update posting: %w", err)
	}

	result, err := scanPosting(s.db.QueryRowContext(ctx, `
		UPDATE postings
		SET company = ?, 
			title = ?, 
			url = ?,
			normalized_url = ?, 
			description = ?,
			location = ?, 
			employment_type = ?, 
			workplace_arrangement = ?,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
		RETURNING `+postingColumns,
		validated.Company,
		validated.Title,
		validated.URL,
		validated.NormalizedURL,
		validated.Description,
		validated.Location,
		validated.EmploymentType,
		validated.WorkplaceArrangement,
		id,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return Posting{}, ErrNotFound
	}
	if err != nil {
		return Posting{}, fmt.Errorf(
			"update posting: %w",
			s.classifyWriteError(ctx, err, validated.NormalizedURL, id),
		)
	}

	return result, nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM postings WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("delete posting: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read posting deletion result: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}

	return nil
}

func validateListOptions(options ListOptions) error {
	fieldErrors := ValidationErrors{}

	switch options.EmploymentType {
	case "", FilterUnknown, EmploymentFullTime, EmploymentOther:
	default:
		fieldErrors["employment_type"] =
			"Choose All, Full-time, Other, or Unknown."
	}

	switch options.WorkplaceArrangement {
	case "", FilterUnknown, WorkplaceRemote, WorkplaceHybrid, WorkplaceOnSite:
	default:
		fieldErrors["workplace_arrangement"] =
			"Choose All, Remote, Hybrid, On-site, or Unknown."
	}

	if len(fieldErrors) > 0 {
		return fieldErrors
	}

	return nil
}

func (s *Store) List(
	ctx context.Context,
	options ListOptions,
) ([]Summary, error) {
	if err := validateListOptions(options); err != nil {
		return nil, err
	}

	query := `
		SELECT id, company, title, location,
			employment_type, workplace_arrangement
		FROM postings
		WHERE 1=1
	`

	var args []any

	search := strings.TrimSpace(options.Query)
	if search != "" {
		query += `
			AND (
				instr(lower(company), lower(?)) > 0
				OR instr(lower(title), lower(?)) > 0
				OR instr(lower(description), lower(?)) > 0
			)
		`
		args = append(args, search, search, search)
	}

	switch options.EmploymentType {
	case "":
	case FilterUnknown:
		query += " AND employment_type IS NULL"
	default:
		query += " AND employment_type = ?"
		args = append(args, options.EmploymentType)
	}

	switch options.WorkplaceArrangement {
	case "":
	case FilterUnknown:
		query += " AND workplace_arrangement IS NULL"
	default:
		query += " AND workplace_arrangement = ?"
		args = append(args, options.WorkplaceArrangement)
	}

	query += " ORDER BY created_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list postings: %w", err)
	}
	defer rows.Close()

	items := make([]Summary, 0)

	for rows.Next() {
		var item Summary

		if err := rows.Scan(
			&item.ID,
			&item.Company,
			&item.Title,
			&item.Location,
			&item.EmploymentType,
			&item.WorkplaceArrangement,
		); err != nil {
			return nil, fmt.Errorf("scan posting summary: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posting summaries: %w", err)
	}

	return items, nil
}
