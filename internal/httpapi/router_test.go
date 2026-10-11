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

	assetsDir := filepath.Join(webDir, "assets")
	if err := os.Mkdir(assetsDir, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(assetsDir, "app.css"),
		[]byte("body { color: navy; }\n"),
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
		{"postings page", "GET", "/postings", "", 200, "text/html", "JST"},
		{"capture page", "GET", "/postings/new", "", 200, "text/html", "JST"},
		{"detail page", "GET", "/postings/123", "", 200, "text/html", "JST"},
		{"edit page", "GET", "/postings/123/edit", "", 200, "text/html", "JST"},
		{"status page", "GET", "/status", "", 200, "text/html", "JST"},
		{"page query", "GET", "/postings?q=engineer", "", 200, "text/html", "JST"},

		{"root HEAD", "HEAD", "/", "", 200, "text/html", ""},
		{"postings HEAD", "HEAD", "/postings", "", 200, "text/html", ""},
		{"capture HEAD", "HEAD", "/postings/new", "", 200, "text/html", ""},
		{"detail HEAD", "HEAD", "/postings/123", "", 200, "text/html", ""},
		{"edit HEAD", "HEAD", "/postings/123/edit", "", 200, "text/html", ""},
		{"status HEAD", "HEAD", "/status", "", 200, "text/html", ""},

		{"existing asset", "GET", "/assets/app.css", "", 200, "text/css", "body { color: navy; }"},
		{"existing asset HEAD", "HEAD", "/assets/app.css", "", 200, "text/css", ""},
		{"missing JavaScript", "GET", "/assets/missing.js", "", 404, "text/plain", "404 page not found"},
		{"missing CSS", "GET", "/assets/missing.css", "", 404, "text/plain", "404 page not found"},
		{"missing favicon", "GET", "/favicon.ico", "", 404, "text/plain", "404 page not found"},
		{"missing asset HEAD", "HEAD", "/assets/missing.js", "", 404, "text/plain", ""},

		{"unknown page", "GET", "/postngs", "", 404, "text/plain", "404 page not found"},
		{"unknown nested page", "GET", "/postings/123/unknown", "", 404, "text/plain", "404 page not found"},
		{"unknown page HEAD", "HEAD", "/postngs", "", 404, "text/plain", ""},
		{"API root", "GET", "/api", "", 404, "application/json", "not_found"},
		{"unknown API prefix", "GET", "/api/missing", "", 404, "application/json", "not_found"},

		{"frontend wrong method", "POST", "/postings", "", 405, "application/json", "method_not_allowed"},
		{"asset wrong method", "POST", "/assets/app.css", "", 405, "application/json", "method_not_allowed"},
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
			if test.method == http.MethodHead &&
				test.status == http.StatusOK &&
				response.Body.Len() != 0 {
				t.Fatalf("HEAD response has a body: %q", response.Body.String())
			}
			if test.contentType == "application/json" && !json.Valid(response.Body.Bytes()) {
				t.Fatal("response is not valid JSON")
			}
			if test.status == 405 && response.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf(
					"Allow = %q, want %q",
					response.Header().Get("Allow"),
					"GET, HEAD",
				)
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

func TestRouterWithoutFrontend(t *testing.T) {
	db, err := storage.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	handler := NewRouter(db, "")

	cases := []struct {
		name   string
		path   string
		status int
		body   string
	}{
		{"health", "/api/v1/health", 200, `"status":"ok"`},
		{"root", "/", 404, "not_found"},
		{"postings page", "/postings", 404, "not_found"},
		{"status page", "/status", 404, "not_found"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(
				http.MethodGet,
				"http://127.0.0.1:8080"+test.path,
				nil,
			)

			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf(
					"status = %d, want %d",
					response.Code,
					test.status,
				)
			}

			if !strings.HasPrefix(
				response.Header().Get("Content-Type"),
				"application/json",
			) {
				t.Fatalf(
					"unexpected content type: %s",
					response.Header().Get("Content-Type"),
				)
			}

			if !json.Valid(response.Body.Bytes()) {
				t.Fatal("response is not valid JSON")
			}

			if !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf(
					"unexpected body: %s",
					response.Body.String(),
				)
			}
		})
	}
}
