package posting

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/DAFinnell/jst/internal/storage"
)

func trackingContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func mustTracking(t *testing.T, store *Store, id int64) Tracking {
	t.Helper()
	result, err := store.GetTracking(trackingContext(t), id)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustSaveTracking(
	t *testing.T,
	store *Store,
	id int64,
	patch TrackingPatch,
) Tracking {
	t.Helper()
	result, err := store.SaveTracking(trackingContext(t), id, patch)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func requirePersonalCounts(t *testing.T, db *sql.DB, cards, events int) {
	t.Helper()
	for _, test := range []struct {
		query string
		want  int
	}{
		{"SELECT COUNT(*) FROM posting_tracking", cards},
		{"SELECT COUNT(*) FROM posting_stage_history", events},
	} {
		var got int
		if err := db.QueryRow(test.query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("%s: got %d, want %d", test.query, got, test.want)
		}
	}
}

func TestTrackingReadInitializationAndChanges(t *testing.T) {
	ctx := trackingContext(t)
	store, db := openPostingStore(t)
	source := createTestPosting(t, store, validInput())

	untracked := mustTracking(t, store, source.ID)
	wantUntracked := Tracking{
		PostingID: source.ID,
		Stage:     StageInbox,
		History:   []StageEvent{},
	}
	if !reflect.DeepEqual(untracked, wantUntracked) {
		t.Fatalf("untracked = %#v", untracked)
	}
	requirePersonalCounts(t, db, 0, 0)

	first := mustSaveTracking(t, store, source.ID, TrackingPatch{
		ApplicationDate:    stringPointer("2026-10-12"),
		ApplicationDateSet: true,
	})
	if first.Stage != StageInbox ||
		first.ApplicationDate == nil || *first.ApplicationDate != "2026-10-12" ||
		first.CreatedAt == nil || first.UpdatedAt == nil ||
		len(first.History) != 1 ||
		first.History[0].FromStage != nil ||
		first.History[0].ToStage != StageInbox {
		t.Fatalf("first save = %#v", first)
	}

	const old = "2000-01-01T00:00:00.000Z"
	if _, err := db.Exec(`
		UPDATE posting_tracking SET updated_at = ? WHERE posting_id = ?
	`, old, source.ID); err != nil {
		t.Fatal(err)
	}
	before := mustTracking(t, store, source.ID)

	unchanged := mustSaveTracking(t, store, source.ID, TrackingPatch{
		Stage:              stringPointer(StageInbox),
		ApplicationDate:    stringPointer("2026-10-12"),
		ApplicationDateSet: true,
	})
	if !reflect.DeepEqual(unchanged, before) {
		t.Fatal("identical save changed tracking or its timestamp")
	}

	corrected := mustSaveTracking(t, store, source.ID, TrackingPatch{
		ApplicationDate:    stringPointer("2026-10-13"),
		ApplicationDateSet: true,
	})
	if corrected.ApplicationDate == nil ||
		*corrected.ApplicationDate != "2026-10-13" ||
		corrected.UpdatedAt == nil || *corrected.UpdatedAt == old ||
		!reflect.DeepEqual(corrected.History, before.History) {
		t.Fatal("date correction did not update only the personal values")
	}

	previous := StageInbox
	var latest Tracking
	for i, stage := range []string{
		StageApplied, StageOffer, StageInterviewing, StageApplied,
	} {
		latest = mustSaveTracking(t, store, source.ID, TrackingPatch{
			Stage: stringPointer(stage),
		})
		event := latest.History[len(latest.History)-1]
		if latest.Stage != stage ||
			latest.ApplicationDate == nil ||
			*latest.ApplicationDate != "2026-10-13" ||
			len(latest.History) != i+2 ||
			event.FromStage == nil || *event.FromStage != previous ||
			event.ToStage != stage {
			t.Fatalf("stage change %d = %#v", i, latest)
		}
		previous = stage
	}

	for i, event := range latest.History {
		stamp, err := time.Parse(time.RFC3339Nano, event.ChangedAt)
		if err != nil || stamp.Location() != time.UTC {
			t.Fatalf("invalid UTC event time: %q", event.ChangedAt)
		}
		if i > 0 && event.ID <= latest.History[i-1].ID {
			t.Fatal("history is not ordered by increasing ID")
		}
	}

	cleared := mustSaveTracking(t, store, source.ID, TrackingPatch{
		ApplicationDateSet: true,
	})
	if cleared.ApplicationDate != nil ||
		cleared.Stage != StageApplied ||
		!reflect.DeepEqual(cleared.History, latest.History) {
		t.Fatal("date clearing changed stage/history or retained the date")
	}

	persistedSource, err := store.Get(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persistedSource, source) {
		t.Fatal("tracking changed source fields or timestamps")
	}

	edited := source.Input
	edited.Title = "Updated source title"
	if _, err := store.Update(ctx, source.ID, edited); err != nil {
		t.Fatal(err)
	}
	if after := mustTracking(t, store, source.ID); !reflect.DeepEqual(after, cleared) {
		t.Fatal("source editing changed personal tracking")
	}
}

func TestTrackingValidationAndMissing(t *testing.T) {
	store, db := openPostingStore(t)
	source := createTestPosting(t, store, validInput())

	for _, stage := range []string{
		StageInbox, StageApplied, StageInterviewing,
		StageOffer, StageRejected, StageClosed,
	} {
		if !validStage(stage) {
			t.Fatalf("supported stage rejected: %q", stage)
		}
	}

	for _, test := range []struct {
		name  string
		patch TrackingPatch
		field string
	}{
		{"empty", TrackingPatch{}, "stage"},
		{"invalid stage", TrackingPatch{
			Stage: stringPointer("screening"),
		}, "stage"},
		{"impossible date", TrackingPatch{
			ApplicationDateSet: true,
			ApplicationDate:    stringPointer("2026-02-30"),
		}, "application_date"},
		{"empty date", TrackingPatch{
			ApplicationDateSet: true,
			ApplicationDate:    stringPointer(""),
		}, "application_date"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.SaveTracking(trackingContext(t), source.ID, test.patch)
			var fields ValidationErrors
			if !errors.As(err, &fields) || fields[test.field] == "" {
				t.Fatalf("error = %v, want field %q", err, test.field)
			}
			requirePersonalCounts(t, db, 0, 0)
		})
	}

	first := mustSaveTracking(t, store, source.ID, TrackingPatch{
		Stage:              stringPointer(StageClosed),
		ApplicationDate:    stringPointer("2024-02-29"),
		ApplicationDateSet: true,
	})
	if first.Stage != StageClosed ||
		len(first.History) != 1 ||
		first.History[0].FromStage != nil ||
		first.History[0].ToStage != StageClosed {
		t.Fatalf("chosen starting stage = %#v", first)
	}

	if _, err := store.GetTracking(trackingContext(t), 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing read error = %v", err)
	}
	if _, err := store.SaveTracking(trackingContext(t), 999999, TrackingPatch{
		Stage: stringPointer(StageApplied),
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing save error = %v", err)
	}
}

func TestCaptureDefaultsValidationAndReopen(t *testing.T) {
	ctx := trackingContext(t)
	dataDir := t.TempDir()
	db, err := storage.Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := NewStore(db)

	first, err := store.Capture(ctx, CaptureInput{Input: validInput()})
	if err != nil {
		t.Fatal(err)
	}
	defaults := mustTracking(t, store, first.ID)
	if defaults.Stage != StageApplied || defaults.ApplicationDate != nil ||
		len(defaults.History) != 1 ||
		defaults.History[0].FromStage != nil ||
		defaults.History[0].ToStage != StageApplied {
		t.Fatalf("capture defaults = %#v", defaults)
	}

	input := validInput()
	input.URL = "https://example.com/jobs/custom-capture"
	custom, err := store.Capture(ctx, CaptureInput{
		Input:           input,
		InitialStage:    stringPointer(StageOffer),
		ApplicationDate: stringPointer("2024-02-29"),
	})
	if err != nil {
		t.Fatal(err)
	}
	customTracking := mustTracking(t, store, custom.ID)
	if customTracking.Stage != StageOffer ||
		customTracking.ApplicationDate == nil ||
		*customTracking.ApplicationDate != "2024-02-29" ||
		len(customTracking.History) != 1 ||
		customTracking.History[0].FromStage != nil ||
		customTracking.History[0].ToStage != StageOffer {
		t.Fatalf("custom capture = %#v", customTracking)
	}

	_, err = store.Capture(ctx, CaptureInput{Input: validInput()})
	requireDuplicate(t, err, first.ID)

	invalid := validInput()
	invalid.Company = ""
	_, err = store.Capture(ctx, CaptureInput{
		Input:           invalid,
		InitialStage:    stringPointer("screening"),
		ApplicationDate: stringPointer("2026-02-30"),
	})
	var fields ValidationErrors
	if !errors.As(err, &fields) ||
		fields["company"] == "" ||
		fields["initial_stage"] == "" ||
		fields["application_date"] == "" {
		t.Fatalf("capture validation = %v", err)
	}
	requirePersonalCounts(t, db, 2, 2)
	requirePostingIDs(t, store, ListOptions{}, custom.ID, first.ID)

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(ctx, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	reopenedStore := NewStore(reopened)

	for _, want := range []Tracking{defaults, customTracking} {
		if got := mustTracking(t, reopenedStore, want.PostingID); !reflect.DeepEqual(got, want) {
			t.Fatal("tracking/history changed after reopening")
		}
	}
}

func TestTrackingHistoryFailureRollsBack(t *testing.T) {
	for _, operation := range []string{"capture", "first save", "stage change"} {
		t.Run(operation, func(t *testing.T) {
			ctx := trackingContext(t)
			store, db := openPostingStore(t)
			source := createTestPosting(t, store, validInput())

			cards, events := 0, 0
			if operation == "stage change" {
				mustSaveTracking(t, store, source.ID, TrackingPatch{
					Stage:              stringPointer(StageApplied),
					ApplicationDateSet: true,
					ApplicationDate:    stringPointer("2026-10-12"),
				})
				cards, events = 1, 1
			}
			before := mustTracking(t, store, source.ID)

			if _, err := db.Exec(`
				CREATE TRIGGER reject_tracking_history
				BEFORE INSERT ON posting_stage_history
				BEGIN
					SELECT RAISE(ABORT, 'forced history failure');
				END
			`); err != nil {
				t.Fatal(err)
			}

			var err error
			if operation == "capture" {
				input := validInput()
				input.URL = "https://example.com/jobs/failed-capture"
				_, err = store.Capture(ctx, CaptureInput{Input: input})
			} else {
				_, err = store.SaveTracking(ctx, source.ID, TrackingPatch{
					Stage:              stringPointer(StageOffer),
					ApplicationDateSet: true,
					ApplicationDate:    stringPointer("2026-10-13"),
				})
			}
			if err == nil {
				t.Fatal("write succeeded despite rejected history insertion")
			}

			requirePostingIDs(t, store, ListOptions{}, source.ID)
			requirePersonalCounts(t, db, cards, events)
			if after := mustTracking(t, store, source.ID); !reflect.DeepEqual(after, before) {
				t.Fatal("failed write changed tracking/history/timestamps")
			}
			afterSource, err := store.Get(ctx, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(afterSource, source) {
				t.Fatal("failed write changed source data")
			}
		})
	}
}

func TestTrackingStageSummariesAndCombinedFilters(t *testing.T) {
	store, db := openPostingStore(t)

	input := validInput()
	input.Title = "Go Developer"
	input.EmploymentType = stringPointer(EmploymentFullTime)
	input.WorkplaceArrangement = stringPointer(WorkplaceRemote)
	untracked := createTestPosting(t, store, input)

	input.URL = "https://example.com/jobs/inbox"
	inbox := createTestPosting(t, store, input)
	mustSaveTracking(t, store, inbox.ID, TrackingPatch{
		Stage: stringPointer(StageInbox),
	})

	input.URL = "https://example.com/jobs/applied"
	applied := createTestPosting(t, store, input)
	mustSaveTracking(t, store, applied.ID, TrackingPatch{
		Stage: stringPointer(StageApplied),
	})

	input.URL = "https://example.com/jobs/other-applied"
	input.EmploymentType = stringPointer(EmploymentOther)
	other := createTestPosting(t, store, input)
	mustSaveTracking(t, store, other.ID, TrackingPatch{
		Stage: stringPointer(StageApplied),
	})

	requirePostingIDs(t, store, ListOptions{
		Stage: StageInbox,
	}, inbox.ID, untracked.ID)

	requirePostingIDs(t, store, ListOptions{
		Query:                "Go",
		EmploymentType:       EmploymentFullTime,
		WorkplaceArrangement: WorkplaceRemote,
		Stage:                StageApplied,
	}, applied.ID)

	requirePostingIDs(t, store, ListOptions{Stage: StageOffer})

	items, err := store.List(trackingContext(t), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		want := StageApplied
		if item.ID == untracked.ID || item.ID == inbox.ID {
			want = StageInbox
		}
		if item.Stage != want {
			t.Fatalf("posting %d stage = %q, want %q", item.ID, item.Stage, want)
		}
	}

	_, err = store.List(trackingContext(t), ListOptions{Stage: "screening"})
	var fields ValidationErrors
	if !errors.As(err, &fields) || fields["stage"] == "" {
		t.Fatalf("invalid filter error = %v", err)
	}
	requirePersonalCounts(t, db, 3, 3)
}
