package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/DAFinnell/jst/internal/posting"
)

type optionalString struct {
	Present bool
	Value   *string
}

func (field *optionalString) UnmarshalJSON(data []byte) error {
	field.Present = true
	return json.Unmarshal(data, &field.Value)
}

type captureFields struct {
	postingFields
	InitialStage    optionalString `json:"initial_stage"`
	ApplicationDate *string        `json:"application_date"`
}

type trackingPatchFields struct {
	Stage           optionalString `json:"stage"`
	ApplicationDate optionalString `json:"application_date"`
}

type stageEventResponse struct {
	ID        int64   `json:"id"`
	FromStage *string `json:"from_stage"`
	ToStage   string  `json:"to_stage"`
	ChangedAt string  `json:"changed_at"`
}

type trackingResponse struct {
	PostingID       int64                `json:"posting_id"`
	Stage           string               `json:"stage"`
	ApplicationDate *string              `json:"application_date"`
	CreatedAt       *string              `json:"created_at"`
	UpdatedAt       *string              `json:"updated_at"`
	History         []stageEventResponse `json:"history"`
}

func toTrackingResponse(saved posting.Tracking) trackingResponse {
	history := make([]stageEventResponse, 0, len(saved.History))
	for _, event := range saved.History {
		history = append(history, stageEventResponse{
			ID:        event.ID,
			FromStage: event.FromStage,
			ToStage:   event.ToStage,
			ChangedAt: event.ChangedAt,
		})
	}

	return trackingResponse{
		PostingID:       saved.PostingID,
		Stage:           saved.Stage,
		ApplicationDate: saved.ApplicationDate,
		CreatedAt:       saved.CreatedAt,
		UpdatedAt:       saved.UpdatedAt,
		History:         history,
	}
}

func decodeCaptureInput(
	w http.ResponseWriter,
	r *http.Request,
) (posting.CaptureInput, bool) {
	fields, ok := decodePostingBody[captureFields](w, r)
	if !ok {
		return posting.CaptureInput{}, false
	}

	if fields.InitialStage.Present && fields.InitialStage.Value == nil {
		writePostingError(w, posting.ValidationErrors{
			"initial_stage": "Choose an initial stage.",
		})
		return posting.CaptureInput{}, false
	}

	return posting.CaptureInput{
		Input:           fields.postingFields.toInput(),
		InitialStage:    fields.InitialStage.Value,
		ApplicationDate: fields.ApplicationDate,
	}, true
}

func (h postingHandler) getTracking(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePostingId(w, r)
	if !ok {
		return
	}

	saved, err := h.store.GetTracking(r.Context(), id)
	if err != nil {
		writePostingError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toTrackingResponse(saved))
}

func (h postingHandler) patchTracking(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePostingId(w, r)
	if !ok {
		return
	}

	fields, ok := decodePostingBody[trackingPatchFields](w, r)
	if !ok {
		return
	}

	if fields.Stage.Present && fields.Stage.Value == nil {
		writePostingError(w, posting.ValidationErrors{
			"stage": "Choose a stage.",
		})
		return
	}

	saved, err := h.store.SaveTracking(r.Context(), id, posting.TrackingPatch{
		Stage:              fields.Stage.Value,
		ApplicationDate:    fields.ApplicationDate.Value,
		ApplicationDateSet: fields.ApplicationDate.Present,
	})
	if err != nil {
		writePostingError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toTrackingResponse(saved))
}
