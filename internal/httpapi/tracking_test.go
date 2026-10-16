package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/DAFinnell/jst/internal/posting"
)

func getHTTPTracking(
	t *testing.T,
	handler http.Handler,
	id int64,
) trackingResponse {
	t.Helper()
	response := postingRequest(t, handler, "GET",
		fmt.Sprintf("/api/v1/postings/%d/tracking", id), "", nil)
	var result trackingResponse
	decodePostingHTTP(t, response, http.StatusOK, &result)
	return result
}

func TestTrackingHTTPFlowAndCaptureCompatibility(t *testing.T) {
	handler, db := openPostingRouter(t)
	store := posting.NewStore(db)
	source, err := store.Create(context.Background(), posting.Input{
		Company:     "Example Company",
		Title:       "Developer",
		URL:         "https://example.com/jobs/untracked-http",
		Description: "Example description",
	})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/v1/postings/%d/tracking", source.ID)

	response := postingRequest(t, handler, "GET", path, "", nil)
	var raw map[string]json.RawMessage
	decodePostingHTTP(t, response, 200, &raw)
	if len(raw) != 6 ||
		string(raw["stage"]) != `"inbox"` ||
		string(raw["application_date"]) != "null" ||
		string(raw["created_at"]) != "null" ||
		string(raw["updated_at"]) != "null" ||
		string(raw["history"]) != "[]" {
		t.Fatalf("untracked response = %s", response.Body.String())
	}

	response = postingRequest(t, handler, "PATCH", path,
		`{"application_date":"2026-10-12"}`, nil)
	var first trackingResponse
	decodePostingHTTP(t, response, 200, &first)
	if first.Stage != "inbox" || len(first.History) != 1 ||
		first.History[0].FromStage != nil ||
		first.History[0].ToStage != "inbox" {
		t.Fatalf("first PATCH = %#v", first)
	}

	response = postingRequest(t, handler, "PATCH", path,
		`{"stage":"applied"}`, nil)
	var applied trackingResponse
	decodePostingHTTP(t, response, 200, &applied)
	if applied.ApplicationDate == nil ||
		*applied.ApplicationDate != "2026-10-12" ||
		len(applied.History) != 2 ||
		applied.History[1].FromStage == nil ||
		*applied.History[1].FromStage != "inbox" ||
		applied.History[1].ToStage != "applied" {
		t.Fatalf("stage-only PATCH = %#v", applied)
	}

	response = postingRequest(t, handler, "PATCH", path,
		`{"application_date":null}`, nil)
	var cleared trackingResponse
	decodePostingHTTP(t, response, 200, &cleared)
	if cleared.ApplicationDate != nil || cleared.Stage != "applied" ||
		!reflect.DeepEqual(cleared.History, applied.History) {
		t.Fatal("null date did not clear independently")
	}

	response = postingRequest(t, handler, "PATCH", path,
		`{"stage":"applied"}`, nil)
	var unchanged trackingResponse
	decodePostingHTTP(t, response, 200, &unchanged)
	if !reflect.DeepEqual(unchanged, cleared) {
		t.Fatal("identical PATCH changed the response state")
	}
	if fetched := getHTTPTracking(t, handler, source.ID); !reflect.DeepEqual(fetched, cleared) {
		t.Fatal("GET differs from saved tracking")
	}

	legacyInput := postingFixture("legacy-capture")
	response = postingRequest(t, handler, "POST", "/api/v1/postings",
		postingJSON(t, legacyInput), nil)
	var legacy postingResponse
	decodePostingHTTP(t, response, 201, &legacy)
	requirePostingJSONShape(t, response.Body.Bytes())
	if response.Header().Get("Location") != fmt.Sprintf("/api/v1/postings/%d", legacy.ID) {
		t.Fatal("capture Location changed")
	}
	legacyTracking := getHTTPTracking(t, handler, legacy.ID)
	if legacyTracking.Stage != "applied" ||
		legacyTracking.ApplicationDate != nil ||
		len(legacyTracking.History) != 1 ||
		legacyTracking.History[0].FromStage != nil {
		t.Fatalf("legacy capture initialization = %#v", legacyTracking)
	}

	customInput := postingFixture("custom-http-capture")
	customInput["initial_stage"] = "interviewing"
	customInput["application_date"] = "2026-10-10"
	custom := createHTTPPosting(t, handler, customInput)
	personal := getHTTPTracking(t, handler, custom.ID)
	if personal.Stage != "interviewing" ||
		personal.ApplicationDate == nil ||
		*personal.ApplicationDate != "2026-10-10" ||
		personal.History[0].ToStage != "interviewing" {
		t.Fatalf("custom capture = %#v", personal)
	}

	replacement := postingFixture("custom-http-capture")
	replacement["title"] = "Updated source title"
	response = postingRequest(t, handler, "PUT",
		fmt.Sprintf("/api/v1/postings/%d", custom.ID),
		postingJSON(t, replacement), nil)
	var edited postingResponse
	decodePostingHTTP(t, response, 200, &edited)
	if edited.Title != "Updated source title" {
		t.Fatal("source edit did not save")
	}
	if after := getHTTPTracking(t, handler, custom.ID); !reflect.DeepEqual(after, personal) {
		t.Fatal("HTTP source edit changed tracking")
	}
}

func TestTrackingHTTPBoundaries(t *testing.T) {
	handler, _ := openPostingRouter(t)
	saved := createHTTPPosting(t, handler, postingFixture("tracking-errors"))
	path := fmt.Sprintf("/api/v1/postings/%d/tracking", saved.ID)
	before := getHTTPTracking(t, handler, saved.ID)

	for _, test := range []struct {
		name    string
		body    string
		headers map[string]string
		status  int
		code    string
	}{
		{"empty patch", `{}`, nil, 400, "validation_failed"},
		{"null stage", `{"stage":null}`, nil, 400, "validation_failed"},
		{"invalid stage", `{"stage":"screening"}`, nil, 400, "validation_failed"},
		{"invalid date", `{"application_date":"2026-02-30"}`, nil, 400, "validation_failed"},
		{"empty date", `{"application_date":""}`, nil, 400, "validation_failed"},
		{"wrong stage type", `{"stage":5}`, nil, 400, "invalid_json"},
		{"wrong date type", `{"application_date":false}`, nil, 400, "invalid_json"},
		{"unknown field", `{"history":[]}`, nil, 400, "invalid_json"},
		{"null object", `null`, nil, 400, "invalid_json"},
		{"extra object", `{"stage":"offer"}{}`, nil, 400, "invalid_json"},
		{"content type", `{"stage":"offer"}`,
			map[string]string{"Content-Type": "text/plain"}, 415, "unsupported_media_type"},
		{"body limit", `{"application_date":"` + strings.Repeat("x", 1<<20) + `"}`,
			nil, 413, "payload_too_large"},
		{"cross origin", `{"stage":"offer"}`,
			map[string]string{"Origin": "https://example.org"}, 403, "cross_origin_forbidden"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := postingRequest(t, handler, "PATCH", path, test.body, test.headers)
			requirePostingHTTPError(t, response, test.status, test.code)
			if after := getHTTPTracking(t, handler, saved.ID); !reflect.DeepEqual(after, before) {
				t.Fatal("rejected request changed tracking")
			}
		})
	}

	for _, method := range []string{"GET", "PATCH"} {
		response := postingRequest(t, handler, method,
			"/api/v1/postings/nope/tracking", `{"stage":"offer"}`, nil)
		requirePostingHTTPError(t, response, 400, "invalid_id")

		response = postingRequest(t, handler, method,
			"/api/v1/postings/999999/tracking", `{"stage":"offer"}`, nil)
		requirePostingHTTPError(t, response, 404, "not_found")
	}

	response := postingRequest(t, handler, "PUT", path, `{}`, nil)
	requirePostingHTTPError(t, response, 405, "method_not_allowed")
	if response.Header().Get("Allow") != "GET, HEAD, PATCH" {
		t.Fatal("unexpected tracking Allow header")
	}

	input := postingFixture("null-initial-stage")
	input["initial_stage"] = nil
	response = postingRequest(t, handler, "POST", "/api/v1/postings",
		postingJSON(t, input), nil)
	result := requirePostingHTTPError(t, response, 400, "validation_failed")
	if result.FieldErrors["initial_stage"] == "" {
		t.Fatal("missing initial-stage field error")
	}

	for _, field := range []string{"initial_stage", "application_date"} {
		input := postingFixture("tracking-errors")
		if field == "initial_stage" {
			input[field] = "offer"
		} else {
			input[field] = "2026-10-12"
		}
		response := postingRequest(t, handler, "PUT",
			fmt.Sprintf("/api/v1/postings/%d", saved.ID),
			postingJSON(t, input), nil)
		requirePostingHTTPError(t, response, 400, "invalid_json")
	}

	closedHandler, db := openPostingRouter(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "PATCH"} {
		response := postingRequest(t, closedHandler, method,
			"/api/v1/postings/1/tracking", `{"stage":"offer"}`, nil)
		requirePostingHTTPError(t, response, 500, "internal_error")
	}
}

func TestTrackingHTTPStageFilter(t *testing.T) {
	handler, db := openPostingRouter(t)

	inboxInput := postingFixture("filter-inbox")
	inboxInput["initial_stage"] = "inbox"
	inbox := createHTTPPosting(t, handler, inboxInput)

	appliedInput := postingFixture("filter-applied")
	appliedInput["title"] = "Go Developer"
	appliedInput["employment_type"] = "full_time"
	appliedInput["workplace_arrangement"] = "remote"
	applied := createHTTPPosting(t, handler, appliedInput)

	untracked, err := posting.NewStore(db).Create(context.Background(), posting.Input{
		Company:     "Untracked Example",
		Title:       "Developer",
		URL:         "https://example.com/jobs/filter-untracked",
		Description: "Example description",
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		query string
		ids   []int64
		stage string
	}{
		{"stage=inbox", []int64{untracked.ID, inbox.ID}, "inbox"},
		{"stage=applied&q=Go&employment_type=full_time&workplace_arrangement=remote",
			[]int64{applied.ID}, "applied"},
		{"stage=offer", nil, "offer"},
	} {
		response := postingRequest(t, handler, "GET",
			"/api/v1/postings?"+test.query, "", nil)
		var result postingListResponse
		decodePostingHTTP(t, response, 200, &result)
		if result.Items == nil || len(result.Items) != len(test.ids) {
			t.Fatalf("query %q returned %#v", test.query, result.Items)
		}
		for i, id := range test.ids {
			if result.Items[i].ID != id || result.Items[i].Stage != test.stage {
				t.Fatalf("query %q item %d = %#v", test.query, i, result.Items[i])
			}
		}
	}

	response := postingRequest(t, handler, "GET",
		"/api/v1/postings?stage=screening", "", nil)
	result := requirePostingHTTPError(t, response, 400, "validation_failed")
	if result.FieldErrors["stage"] == "" {
		t.Fatal("invalid stage filter has no field error")
	}
}
