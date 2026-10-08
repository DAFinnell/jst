package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/DAFinnell/jst/internal/httpapi"
)

func main() {
	addr := os.Getenv("JST_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("JST listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
