package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DAFinnell/jst/internal/storage"
)

func TestRouter(t *testing.T) {
	db, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	webDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(webDir, "index.html"),
		[]byte("<!doctype html><title>JST</title>"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	handler := NewRouter(db, webDir)

	cases := []struct {
		name        string
		method      string
		path        string
		origin      string
		status      int
		contentType string
		body        string
	}{
		{"health", "GET", "/api/v1/health", "", 200, "application/json", `"status":"ok"`},
		{"wrong method", "POST", "/api/v1/health", "", 405, "application/json", "method_not_allowed"},
		{"same origin", "POST", "/api/v1/health", "http://127.0.0.1:8080", 405, "application/json", "method_not_allowed"},
		{"cross origin", "POST", "/api/v1/health", "https://example.com", 403, "application/json", "cross_origin_forbidden"},
		{"unknown API", "GET", "/api/v1/missing", "", 404, "application/json", "not_found"},
		{"frontend", "GET", "/", "", 200, "text/html", "JST"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(
				test.method,
				"http://127.0.0.1:8080"+test.path,
				nil,
			)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			if !strings.HasPrefix(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("unexpected content type: %s", response.Header().Get("Content-Type"))
			}
			if !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("unexpected body: %s", response.Body.String())
			}
			if test.contentType == "application/json" && !json.Valid(response.Body.Bytes()) {
				t.Fatal("response is not valid JSON")
			}
			if test.status == 405 && response.Header().Get("Allow") != "GET, HEAD" {
				t.Fatal("method error is missing the expected Allow header")
			}
		})
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest("GET", "http://127.0.0.1:8080/api/v1/health", nil),
	)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed database status = %d, want 503", response.Code)
	}
	if !strings.Contains(response.Body.String(), "database_unavailable") {
		t.Fatal("missing database error code")
	}
}
