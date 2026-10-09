package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/DAFinnell/jst/internal/httpapi"
	"github.com/DAFinnell/jst/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	addr := os.Getenv("JST_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	dataDir := os.Getenv("JST_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	webDir := os.Getenv("JST_WEB_DIR")
	if webDir != "" {
		info, err := os.Stat(filepath.Join(webDir, "index.html"))
		if err != nil || info.IsDir() {
			return fmt.Errorf("frontend index.html is missing from %q", webDir)
		}
	}

	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	db, err := storage.Open(startupCtx, dataDir)
	cancelStartup()
	if err != nil {
		return err
	}
	defer db.Close()

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(db, webDir),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	log.Printf("JST listening on %s; data directory: %s", addr, dataDir)

	select {
	case err := <-serverErr:
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancelShutdown()

		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
			return fmt.Errorf("shut down HTTP: %w", err)
		}

		return nil
	}
}
