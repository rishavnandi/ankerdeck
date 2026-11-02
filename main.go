// Command ankerdeck serves the AnkerGames top and trending charts with
// CrossOver compatibility badges.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	seedPath := flag.String("seed", "", "Import a chart snapshot JSON into the configured database and exit")
	flag.Parse()
	addr := env("ADDR", ":"+env("PORT", "8080"))
	hosted := os.Getenv("VERCEL") == "1"

	var database *snapshotDB
	var err error
	if connection := os.Getenv("DATABASE_URL"); connection != "" {
		dbCtx, dbCancel := context.WithTimeout(ctx, 20*time.Second)
		database, err = openPostgresSnapshotDB(dbCtx, connection)
		dbCancel()
	} else if hosted {
		return fmt.Errorf("DATABASE_URL is required on Vercel")
	} else {
		database, err = openSnapshotDB(env("DB_PATH", "ankerdeck.db"))
	}
	if err != nil {
		return err
	}
	defer database.close()
	if *seedPath != "" {
		snapshot, err := importSnapshot(ctx, database, *seedPath)
		if err != nil {
			return err
		}
		log.Info("snapshot imported", "top", len(snapshot.Top), "trending", len(snapshot.Trending))
		return nil
	}

	snapshot := Snapshot{}
	if !hosted {
		snapshot, err = database.latest(ctx)
		if err != nil {
			return err
		}
	}
	app, err := NewApp(snapshot)
	if err != nil {
		return err
	}

	var handler http.Handler = app.Handler()
	writeTimeout := 30 * time.Second
	if hosted {
		handler = securityHeaders(&hostedHandler{app: app, database: database, log: log})
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "url", "http://localhost"+addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func readSnapshot(path string) (Snapshot, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return Snapshot{}, err
	}
	if len(snapshot.Top) == 0 || len(snapshot.Trending) == 0 {
		return Snapshot{}, fmt.Errorf("seed must contain both top and trending games")
	}
	return snapshot, nil
}

func importSnapshot(ctx context.Context, database *snapshotDB, path string) (Snapshot, error) {
	snapshot, err := readSnapshot(path)
	if err != nil {
		return Snapshot{}, err
	}
	if err := database.save(ctx, snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
