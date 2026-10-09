package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type healthResponse struct {
	Status string `json:"status"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func NewRouter(db *sql.DB, webDir string) http.Handler {
	api := http.NewServeMux()

	api.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable,
				"database_unavailable", "Database is not available.")
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})

	api.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed,
			"method_not_allowed", "Method is not allowed.")
	})

	api.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound,
			"not_found", "Endpoint was not found.")
	})

	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/api", api)

	if webDir == "" {
		mux.Handle("/", api)
	} else {
		files := http.FileServer(http.Dir(webDir))

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				writeError(w, http.StatusMethodNotAllowed,
					"method_not_allowed", "Method is not allowed.")
				return
			}

			files.ServeHTTP(w, r)
		})
	}

	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusForbidden,
			"cross_origin_forbidden", "Cross origin mutation not allowed.")
	}))

	return protection.Handler(mux)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}
