package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"

	"github.com/DAFinnell/jst/internal/posting"
)

const maxPostingBodyBytes = 1 << 20

type postingFields struct {
	Company              string  `json:"company"`
	Title                string  `json:"title"`
	URL                  string  `json:"url"`
	Description          string  `json:"description"`
	Location             *string `json:"location"`
	EmploymentType       *string `json:"employment_type"`
	WorkplaceArrangement *string `json:"workplace_arrangement"`
}

type postingResponse struct {
	postingFields
	ID        int64  `json:"id"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type postingSummaryResponse struct {
	ID                   int64   `json:"id"`
	Company              string  `json:"company"`
	Title                string  `json:"title"`
	Location             *string `json:"location"`
	EmploymentType       *string `json:"employment_type"`
	WorkplaceArrangement *string `json:"workplace_arrangement"`
}

type postingListResponse struct {
	Items []postingSummaryResponse `json:"items"`
}

type postingHandler struct {
	store *posting.Store
}

func (fields postingFields) toInput() posting.Input {
	return posting.Input{
		Company:              fields.Company,
		Title:                fields.Title,
		URL:                  fields.URL,
		Description:          fields.Description,
		Location:             fields.Location,
		EmploymentType:       fields.EmploymentType,
		WorkplaceArrangement: fields.WorkplaceArrangement,
	}
}

func toPostingResponse(saved posting.Posting) postingResponse {
	return postingResponse{
		postingFields: postingFields{
			Company:              saved.Company,
			Title:                saved.Title,
			URL:                  saved.URL,
			Description:          saved.Description,
			Location:             saved.Location,
			EmploymentType:       saved.EmploymentType,
			WorkplaceArrangement: saved.WorkplaceArrangement,
		},
		ID:        saved.ID,
		Source:    saved.Source,
		CreatedAt: saved.CreatedAt,
		UpdatedAt: saved.UpdatedAt,
	}
}

func decodePostingInput(
	w http.ResponseWriter,
	r *http.Request,
) (posting.Input, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType,
			"unsupported_media_type", "Use application/json for posting requests.")
		return posting.Input{}, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostingBodyBytes)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var fields *postingFields
	if err := decoder.Decode(&fields); err != nil {
		writePostingBodyError(w, err)
		return posting.Input{}, false
	}
	if fields == nil {
		writePostingBodyError(w, nil)
		return posting.Input{}, false
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writePostingBodyError(w, err)
		return posting.Input{}, false
	}

	return fields.toInput(), true
}

func writePostingBodyError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge,
			"payload_too_large", "Posting requests must be no larger than 1 MB.")
		return
	}

	writeError(w, http.StatusBadRequest,
		"invalid_json",
		"Send one JSON object with valid posting field types and no extra fields.")
}

func parsePostingId(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest,
			"invalid_id", "Use a positive numeric posting ID.")
		return 0, false
	}

	return id, true
}

func writePostingError(w http.ResponseWriter, err error) {
	var fieldErrors posting.ValidationErrors
	var duplicate *posting.DuplicateURLError

	switch {
	case errors.As(err, &fieldErrors):
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: apiError{
				Code:        "validation_failed",
				Message:     "Check the highlighted fields.",
				FieldErrors: fieldErrors,
			},
		})

	case errors.As(err, &duplicate):
		writeJSON(w, http.StatusConflict, errorResponse{
			Error: apiError{
				Code:              "duplicate_url",
				Message:           duplicate.Error(),
				ExistingPostingID: duplicate.ExistingPostingID,
			},
		})

	case errors.Is(err, posting.ErrNotFound):
		writeError(w, http.StatusNotFound,
			"not_found", "Posting was not found.")

	default:
		log.Printf("posting request failed: %v", err)
		writeError(w, http.StatusInternalServerError,
			"internal_error", "Unable to complete the posting request.")
	}
}

func (h postingHandler) create(w http.ResponseWriter, r *http.Request) {
	input, ok := decodePostingInput(w, r)
	if !ok {
		return
	}

	saved, err := h.store.Create(r.Context(), input)
	if err != nil {
		writePostingError(w, err)
		return
	}

	w.Header().Set(
		"Location",
		"/api/v1/postings/"+strconv.FormatInt(saved.ID, 10),
	)
	writeJSON(w, http.StatusCreated, toPostingResponse(saved))
}

func (h postingHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePostingId(w, r)
	if !ok {
		return
	}

	saved, err := h.store.Get(r.Context(), id)
	if err != nil {
		writePostingError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostingResponse(saved))
}

func (h postingHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePostingId(w, r)
	if !ok {
		return
	}

	input, ok := decodePostingInput(w, r)
	if !ok {
		return
	}

	saved, err := h.store.Update(r.Context(), id, input)
	if err != nil {
		writePostingError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPostingResponse(saved))
}

func (h postingHandler) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePostingId(w, r)
	if !ok {
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		writePostingError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h postingHandler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	summaries, err := h.store.List(r.Context(), posting.ListOptions{
		Query:                query.Get("q"),
		EmploymentType:       query.Get("employment_type"),
		WorkplaceArrangement: query.Get("workplace_arrangement"),
	})
	if err != nil {
		writePostingError(w, err)
		return
	}

	items := make([]postingSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, postingSummaryResponse{
			ID:                   summary.ID,
			Company:              summary.Company,
			Title:                summary.Title,
			Location:             summary.Location,
			EmploymentType:       summary.EmploymentType,
			WorkplaceArrangement: summary.WorkplaceArrangement,
		})
	}

	writeJSON(w, http.StatusOK, postingListResponse{Items: items})
}
