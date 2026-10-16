package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DAFinnell/jst/internal/storage"
)

func openPostingRouter(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()

	db, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
	})

	return NewRouter(db, ""), db
}

func postingRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(
		method,
		"http://127.0.0.1:8080"+path,
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func postingJSON(t *testing.T, value any) string {
	t.Helper()

	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func decodePostingHTTP(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantStatus int,
	target any,
) {
	t.Helper()

	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s",
			response.Code, wantStatus, response.Body.String())
	}

	if wantStatus == http.StatusNoContent {
		if response.Body.Len() != 0 {
			t.Fatalf("204 response has a body: %s", response.Body.String())
		}
		return
	}

	if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected content type: %q",
			response.Header().Get("Content-Type"))
	}

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body = %s", err, response.Body.String())
	}
}

func requirePostingHTTPError(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) apiError {
	t.Helper()

	var result errorResponse
	decodePostingHTTP(t, response, wantStatus, &result)
	if result.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", result.Error.Code, wantCode)
	}
	if result.Error.Message == "" {
		t.Fatal("error response has no message")
	}

	return result.Error
}

func postingFixture(job string) map[string]any {
	return map[string]any{
		"company":     "Example Company",
		"title":       "Junior Developer",
		"url":         "https://example.com/jobs/" + job,
		"description": "First line\nSecond line",
	}
}

func createHTTPPosting(
	t *testing.T,
	handler http.Handler,
	input map[string]any,
) postingResponse {
	t.Helper()

	response := postingRequest(t, handler, "POST", "/api/v1/postings",
		postingJSON(t, input), nil)

	var result postingResponse
	decodePostingHTTP(t, response, http.StatusCreated, &result)
	return result
}

func getHTTPPosting(
	t *testing.T,
	handler http.Handler,
	id int64,
) postingResponse {
	t.Helper()

	response := postingRequest(t, handler, "GET",
		fmt.Sprintf("/api/v1/postings/%d", id), "", nil)

	var result postingResponse
	decodePostingHTTP(t, response, http.StatusOK, &result)
	return result
}

func requirePostingJSONShape(t *testing.T, body []byte) {
	t.Helper()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{
		"id", "company", "title", "url", "description",
		"location", "employment_type", "workplace_arrangement",
		"source", "created_at", "updated_at",
	}
	if len(fields) != len(wantKeys) {
		t.Fatalf("unexpected posting keys: %s", body)
	}
	for _, key := range wantKeys {
		if _, ok := fields[key]; !ok {
			t.Fatalf("response is missing %q: %s", key, body)
		}
	}
}

func TestPostingHTTPCRUD(t *testing.T) {
	handler, db := openPostingRouter(t)

	input := postingFixture("crud")
	input["company"] = "  Example Company  "
	input["title"] = "\tJunior Developer\n"
	input["url"] = " HTTPS://Example.COM/jobs/crud?utm_source=test#apply "
	input["description"] = "\n  <p>First line</p>\n**Second line**  \n"
	input["location"] = "  Boston, MA  "
	input["employment_type"] = "full_time"
	input["workplace_arrangement"] = "remote"

	response := postingRequest(t, handler, "POST", "/api/v1/postings",
		postingJSON(t, input),
		map[string]string{"Content-Type": "application/json; charset=utf-8"})

	var created postingResponse
	decodePostingHTTP(t, response, http.StatusCreated, &created)
	requirePostingJSONShape(t, response.Body.Bytes())

	path := fmt.Sprintf("/api/v1/postings/%d", created.ID)
	if response.Header().Get("Location") != path {
		t.Fatalf("Location = %q, want %q", response.Header().Get("Location"), path)
	}
	if created.ID <= 0 || created.Source != "manual" {
		t.Fatalf("unexpected identity/source: %#v", created)
	}
	if created.Company != "Example Company" || created.Title != "Junior Developer" {
		t.Fatalf("short fields were not cleaned: %#v", created)
	}
	if created.URL != "HTTPS://Example.COM/jobs/crud?utm_source=test#apply" {
		t.Fatalf("entered URL was not preserved: %q", created.URL)
	}
	if created.Description != input["description"] {
		t.Fatalf("description changed: %q", created.Description)
	}
	if created.Location == nil || *created.Location != "Boston, MA" ||
		created.EmploymentType == nil || *created.EmploymentType != "full_time" ||
		created.WorkplaceArrangement == nil || *created.WorkplaceArrangement != "remote" {
		t.Fatalf("unexpected optional fields: %#v", created)
	}
	for _, timestamp := range []string{created.CreatedAt, created.UpdatedAt} {
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			t.Fatalf("invalid timestamp %q: %v", timestamp, err)
		}
	}

	if fetched := getHTTPPosting(t, handler, created.ID); !reflect.DeepEqual(fetched, created) {
		t.Fatal("retrieved posting differs from created posting")
	}

	oldUpdatedAt := "2000-01-01T00:00:00.000Z"
	if _, err := db.Exec(
		"UPDATE postings SET updated_at = ? WHERE id = ?",
		oldUpdatedAt, created.ID,
	); err != nil {
		t.Fatal(err)
	}

	replacement := postingFixture("crud")
	replacement["title"] = "Developer II"
	replacement["description"] = "Updated description\nStill plain text."

	response = postingRequest(t, handler, "PUT", path,
		postingJSON(t, replacement), nil)

	var updated postingResponse
	decodePostingHTTP(t, response, http.StatusOK, &updated)
	requirePostingJSONShape(t, response.Body.Bytes())

	if updated.ID != created.ID || updated.Source != created.Source ||
		updated.CreatedAt != created.CreatedAt {
		t.Fatal("replacement changed identity, source, or creation time")
	}
	if updated.UpdatedAt == oldUpdatedAt {
		t.Fatal("replacement did not refresh modification time")
	}
	if updated.Title != replacement["title"] ||
		updated.Description != replacement["description"] {
		t.Fatal("replacement did not save editable fields")
	}
	if updated.Location != nil || updated.EmploymentType != nil ||
		updated.WorkplaceArrangement != nil {
		t.Fatal("omitted optional fields were not cleared")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		"location", "employment_type", "workplace_arrangement",
	} {
		if string(raw[field]) != "null" {
			t.Fatalf("%s = %s, want null", field, raw[field])
		}
	}

	if fetched := getHTTPPosting(t, handler, created.ID); !reflect.DeepEqual(fetched, updated) {
		t.Fatal("replacement was not persisted")
	}

	response = postingRequest(t, handler, "DELETE", path, "", nil)
	decodePostingHTTP(t, response, http.StatusNoContent, nil)

	response = postingRequest(t, handler, "GET", path, "", nil)
	requirePostingHTTPError(t, response, http.StatusNotFound, "not_found")

	response = postingRequest(t, handler, "DELETE", path, "", nil)
	requirePostingHTTPError(t, response, http.StatusNotFound, "not_found")
}

func TestPostingHTTPValidation(t *testing.T) {
	handler, _ := openPostingRouter(t)
	saved := createHTTPPosting(t, handler, postingFixture("validation"))
	path := fmt.Sprintf("/api/v1/postings/%d", saved.ID)

	invalid := map[string]any{
		"company":               " ",
		"title":                 " ",
		"url":                   "",
		"description":           "\n\t",
		"employment_type":       "unknown",
		"workplace_arrangement": "unknown",
	}

	for _, request := range []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/postings"},
		{"PUT", path},
	} {
		response := postingRequest(t, handler, request.method, request.path,
			postingJSON(t, invalid), nil)
		result := requirePostingHTTPError(t, response,
			http.StatusBadRequest, "validation_failed")

		wantFields := []string{
			"company", "title", "url", "description",
			"employment_type", "workplace_arrangement",
		}
		if len(result.FieldErrors) != len(wantFields) {
			t.Fatalf("field errors = %#v", result.FieldErrors)
		}
		for _, field := range wantFields {
			if result.FieldErrors[field] == "" {
				t.Fatalf("missing error for %q", field)
			}
		}
		if result.FieldErrors["company"] != "Enter a company." {
			t.Fatalf("unexpected company error: %q", result.FieldErrors["company"])
		}
		if result.ExistingPostingID != 0 {
			t.Fatal("validation error included a duplicate ID")
		}
	}

	if fetched := getHTTPPosting(t, handler, saved.ID); !reflect.DeepEqual(fetched, saved) {
		t.Fatal("invalid replacement changed stored data")
	}
}

func TestPostingHTTPDuplicateURLs(t *testing.T) {
	handler, _ := openPostingRouter(t)
	first := createHTTPPosting(t, handler, postingFixture("first"))
	second := createHTTPPosting(t, handler, postingFixture("second"))

	duplicate := postingFixture("first")
	duplicate["url"] = "https://example.com/jobs/first?utm_source=test#apply"
	secondPath := fmt.Sprintf("/api/v1/postings/%d", second.ID)

	for _, request := range []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/postings"},
		{"PUT", secondPath},
	} {
		response := postingRequest(t, handler, request.method, request.path,
			postingJSON(t, duplicate), nil)
		result := requirePostingHTTPError(t, response,
			http.StatusConflict, "duplicate_url")

		if result.ExistingPostingID != first.ID {
			t.Fatalf("duplicate ID = %d, want %d",
				result.ExistingPostingID, first.ID)
		}
		if len(result.FieldErrors) != 0 {
			t.Fatal("duplicate response included field errors")
		}
	}

	if fetched := getHTTPPosting(t, handler, second.ID); !reflect.DeepEqual(fetched, second) {
		t.Fatal("conflicting replacement changed stored data")
	}

	response := postingRequest(t, handler, "PUT",
		fmt.Sprintf("/api/v1/postings/%d", first.ID),
		postingJSON(t, duplicate), nil)

	var updated postingResponse
	decodePostingHTTP(t, response, http.StatusOK, &updated)
	if updated.ID != first.ID {
		t.Fatal("editing the same URL changed the posting ID")
	}
}

func TestPostingHTTPList(t *testing.T) {
	handler, _ := openPostingRouter(t)

	response := postingRequest(t, handler, "GET", "/api/v1/postings", "", nil)
	var empty map[string]json.RawMessage
	decodePostingHTTP(t, response, http.StatusOK, &empty)
	if len(empty) != 1 || string(empty["items"]) != "[]" {
		t.Fatalf("empty list = %s", response.Body.String())
	}

	alpha := postingFixture("alpha")
	alpha["company"] = "Example Alpha"
	alpha["title"] = "Backend Developer"
	alpha["description"] = "Go and 100%_ready"
	alpha["employment_type"] = "full_time"
	alpha["workplace_arrangement"] = "remote"
	a := createHTTPPosting(t, handler, alpha)

	beta := postingFixture("beta")
	beta["company"] = "Example Beta"
	beta["title"] = "Frontend Developer"
	beta["description"] = "React"
	beta["employment_type"] = "other"
	beta["workplace_arrangement"] = "hybrid"
	b := createHTTPPosting(t, handler, beta)

	gamma := postingFixture("gamma")
	gamma["company"] = "Example Gamma"
	gamma["title"] = "Analyst"
	gamma["description"] = "Go support"
	gamma["workplace_arrangement"] = "on_site"
	c := createHTTPPosting(t, handler, gamma)

	delta := postingFixture("delta")
	delta["company"] = "Example Delta"
	delta["title"] = "Intern"
	delta["description"] = "Research"
	d := createHTTPPosting(t, handler, delta)

	cases := []struct {
		name  string
		query string
		ids   []int64
	}{
		{"all", "", []int64{d.ID, c.ID, b.ID, a.ID}},
		{"title", "q=developer", []int64{b.ID, a.ID}},
		{"company", "q=BETA", []int64{b.ID}},
		{"description", "q=GO", []int64{c.ID, a.ID}},
		{"literal characters", "q=100%25_ready", []int64{a.ID}},
		{"trimmed search", "q=%20GO%20", []int64{c.ID, a.ID}},
		{"blank search", "q=%20", []int64{d.ID, c.ID, b.ID, a.ID}},
		{"no matches", "q=missing", nil},
		{"full time", "employment_type=full_time", []int64{a.ID}},
		{"other", "employment_type=other", []int64{b.ID}},
		{"unknown employment", "employment_type=unknown", []int64{d.ID, c.ID}},
		{"remote", "workplace_arrangement=remote", []int64{a.ID}},
		{"hybrid", "workplace_arrangement=hybrid", []int64{b.ID}},
		{"on site", "workplace_arrangement=on_site", []int64{c.ID}},
		{"unknown workplace", "workplace_arrangement=unknown", []int64{d.ID}},
		{"combined", "q=GO&employment_type=full_time&workplace_arrangement=remote",
			[]int64{a.ID}},
		{"empty filters", "employment_type=&workplace_arrangement=",
			[]int64{d.ID, c.ID, b.ID, a.ID}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := postingRequest(t, handler, "GET",
				"/api/v1/postings?"+test.query, "", nil)

			var result postingListResponse
			decodePostingHTTP(t, response, http.StatusOK, &result)
			if result.Items == nil {
				t.Fatal("items is null")
			}
			if len(result.Items) != len(test.ids) {
				t.Fatalf("items = %#v, want IDs %v", result.Items, test.ids)
			}
			for index, id := range test.ids {
				if result.Items[index].ID != id {
					t.Fatalf("item %d ID = %d, want %d",
						index, result.Items[index].ID, id)
				}
			}
		})
	}

	response = postingRequest(t, handler, "GET", "/api/v1/postings", "", nil)
	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	decodePostingHTTP(t, response, http.StatusOK, &raw)

	wantKeys := []string{
		"id", "company", "title", "location",
		"employment_type", "workplace_arrangement", "stage",
	}
	for _, item := range raw.Items {
		if len(item) != len(wantKeys) {
			t.Fatalf("unexpected summary fields: %#v", item)
		}
		for _, key := range wantKeys {
			if _, ok := item[key]; !ok {
				t.Fatalf("summary is missing %q", key)
			}
		}
	}

	response = postingRequest(t, handler, "GET",
		"/api/v1/postings?employment_type=bad&workplace_arrangement=bad", "", nil)
	result := requirePostingHTTPError(t, response,
		http.StatusBadRequest, "validation_failed")
	if len(result.FieldErrors) != 2 ||
		result.FieldErrors["employment_type"] == "" ||
		result.FieldErrors["workplace_arrangement"] == "" {
		t.Fatalf("unexpected filter errors: %#v", result.FieldErrors)
	}
}

func TestPostingHTTPBodyErrors(t *testing.T) {
	handler, _ := openPostingRouter(t)
	saved := createHTTPPosting(t, handler, postingFixture("body-errors"))
	path := fmt.Sprintf("/api/v1/postings/%d", saved.ID)
	valid := postingJSON(t, postingFixture("body-errors"))

	unknown := postingFixture("body-errors")
	unknown["employmnt_type"] = "full_time"

	wrongType := postingFixture("body-errors")
	wrongType["company"] = 123

	tooLarge := postingFixture("body-errors")
	tooLarge["description"] = strings.Repeat("x", 1<<20)

	cases := []struct {
		name        string
		body        string
		contentType string
		status      int
		code        string
	}{
		{"empty", "", "application/json", 400, "invalid_json"},
		{"malformed", `{"company":`, "application/json", 400, "invalid_json"},
		{"null", `null`, "application/json", 400, "invalid_json"},
		{"array", `[]`, "application/json", 400, "invalid_json"},
		{"string", `"posting"`, "application/json", 400, "invalid_json"},
		{"extra value", valid + `{}`, "application/json", 400, "invalid_json"},
		{"trailing garbage", valid + `oops`, "application/json", 400, "invalid_json"},
		{"unknown field", postingJSON(t, unknown), "application/json", 400, "invalid_json"},
		{"wrong type", postingJSON(t, wrongType), "application/json", 400, "invalid_json"},
		{"missing content type", valid, "", 415, "unsupported_media_type"},
		{"wrong content type", valid, "text/plain", 415, "unsupported_media_type"},
		{"oversized object", postingJSON(t, tooLarge), "application/json", 413, "payload_too_large"},
		{"oversized trailing whitespace", valid + strings.Repeat(" ", 1<<20),
			"application/json", 413, "payload_too_large"},
	}

	for _, test := range cases {
		for _, request := range []struct {
			method string
			path   string
		}{
			{"POST", "/api/v1/postings"},
			{"PUT", path},
		} {
			t.Run(test.name+"/"+request.method, func(t *testing.T) {
				response := postingRequest(t, handler, request.method, request.path,
					test.body, map[string]string{"Content-Type": test.contentType})
				requirePostingHTTPError(t, response, test.status, test.code)
			})
		}
	}

	if fetched := getHTTPPosting(t, handler, saved.ID); !reflect.DeepEqual(fetched, saved) {
		t.Fatal("rejected bodies changed stored data")
	}

	response := postingRequest(t, handler, "GET", "/api/v1/postings", "", nil)
	var listed postingListResponse
	decodePostingHTTP(t, response, http.StatusOK, &listed)
	if len(listed.Items) != 1 || listed.Items[0].ID != saved.ID {
		t.Fatal("rejected bodies created postings")
	}
}

func TestPostingHTTPBodyLimitBoundary(t *testing.T) {
	handler, _ := openPostingRouter(t)
	body := postingJSON(t, postingFixture("boundary"))
	body += strings.Repeat(" ", (1<<20)-len(body))

	response := postingRequest(t, handler, "POST", "/api/v1/postings", body, nil)
	var saved postingResponse
	decodePostingHTTP(t, response, http.StatusCreated, &saved)

	if saved.ID <= 0 {
		t.Fatal("request at the body limit did not create a posting")
	}
}

func TestPostingHTTPIDsAndMethods(t *testing.T) {
	handler, _ := openPostingRouter(t)
	body := postingJSON(t, postingFixture("missing"))

	cases := []struct {
		method string
		path   string
		status int
		code   string
		allow  string
	}{
		{"GET", "/api/v1/postings/nope", 400, "invalid_id", ""},
		{"PUT", "/api/v1/postings/0", 400, "invalid_id", ""},
		{"DELETE", "/api/v1/postings/-1", 400, "invalid_id", ""},
		{"GET", "/api/v1/postings/999999", 404, "not_found", ""},
		{"PUT", "/api/v1/postings/999999", 404, "not_found", ""},
		{"DELETE", "/api/v1/postings/999999", 404, "not_found", ""},
		{"PATCH", "/api/v1/postings", 405, "method_not_allowed", "GET, HEAD, POST"},
		{"POST", "/api/v1/postings/1", 405, "method_not_allowed", "GET, HEAD, PUT, DELETE"},
		{"GET", "/api/v1/postings/1/missing", 404, "not_found", ""},
	}

	for _, test := range cases {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := postingRequest(t, handler, test.method, test.path, body, nil)
			requirePostingHTTPError(t, response, test.status, test.code)
			if response.Header().Get("Allow") != test.allow {
				t.Fatalf("Allow = %q, want %q",
					response.Header().Get("Allow"), test.allow)
			}
		})
	}
}

func TestPostingHTTPDatabaseFailures(t *testing.T) {
	handler, db := openPostingRouter(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	body := postingJSON(t, postingFixture("closed-db"))
	for _, request := range []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/postings"},
		{"GET", "/api/v1/postings"},
		{"GET", "/api/v1/postings/1"},
		{"PUT", "/api/v1/postings/1"},
		{"DELETE", "/api/v1/postings/1"},
	} {
		response := postingRequest(t, handler, request.method, request.path, body, nil)
		result := requirePostingHTTPError(t, response, 500, "internal_error")

		if result.Message != "Unable to complete the posting request." {
			t.Fatalf("unexpected database-failure message: %q", result.Message)
		}
		if len(result.FieldErrors) != 0 || result.ExistingPostingID != 0 {
			t.Fatal("database failure was reported as validation or duplicate details")
		}
	}
}

func TestPostingHTTPCrossOriginProtection(t *testing.T) {
	handler, _ := openPostingRouter(t)
	saved := createHTTPPosting(t, handler, postingFixture("protected"))
	path := fmt.Sprintf("/api/v1/postings/%d", saved.ID)

	input := postingFixture("blocked")
	input["title"] = "Blocked change"
	body := postingJSON(t, input)

	for _, request := range []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/postings"},
		{"PUT", path},
		{"DELETE", path},
	} {
		response := postingRequest(t, handler, request.method, request.path, body,
			map[string]string{"Origin": "https://example.org"})
		requirePostingHTTPError(t, response, 403, "cross_origin_forbidden")
	}

	if fetched := getHTTPPosting(t, handler, saved.ID); !reflect.DeepEqual(fetched, saved) {
		t.Fatal("cross-origin request changed the stored posting")
	}

	response := postingRequest(t, handler, "GET", "/api/v1/postings", "", nil)
	var listed postingListResponse
	decodePostingHTTP(t, response, http.StatusOK, &listed)
	if len(listed.Items) != 1 {
		t.Fatal("cross-origin request created a posting")
	}

	response = postingRequest(t, handler, "POST", "/api/v1/postings", body,
		map[string]string{"Origin": "http://127.0.0.1:8080"})

	var created postingResponse
	decodePostingHTTP(t, response, http.StatusCreated, &created)
	if created.ID == saved.ID {
		t.Fatal("same-origin creation did not create a separate posting")
	}
}
