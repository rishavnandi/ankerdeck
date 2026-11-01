// Command ankerdeck serves the AnkerGames top and trending charts with
// CrossOver compatibility badges.
package main

import (
	"context"
	"errors"
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

	addr := env("ADDR", ":8080")
	interval := envDuration("REFRESH_INTERVAL", 15*time.Minute)
	if interval < time.Minute {
		interval = time.Minute
	}

	database, err := openSnapshotDB(env("DB_PATH", "ankerdeck.db"))
	if err != nil {
		return err
	}
	defer database.close()

	store := NewStore()
	initial, err := database.latest(ctx)
	if err != nil {
		return err
	}
	store.Load(initial)
	fetcher := NewFetcher(os.Getenv("JINA_API_KEY"))
	compat := NewCompatClient(fetcher)
	refresher := NewRefresher(store, database, fetcher, compat, interval, log)
	refreshDone := make(chan struct{})
	go func() {
		defer close(refreshDone)
		refresher.Run(ctx)
	}()
	defer func() {
		stop()
		<-refreshDone
	}()

	app, err := NewApp(store, interval, refresher)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
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

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
