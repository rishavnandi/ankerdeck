package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// hostedHandler keeps all durable state in Postgres. Refresh work runs inside
// an HTTP invocation, rather than depending on a background goroutine surviving.
type hostedHandler struct {
	app      *App
	database *snapshotDB
	fetcher  *Fetcher
	log      *slog.Logger
	interval time.Duration
}

func (h *hostedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" || len(r.URL.Path) >= 8 && r.URL.Path[:8] == "/static/" {
		h.app.Handler().ServeHTTP(w, r)
		return
	}
	cron := r.URL.Path == "/api/cron"
	if cron {
		expected := os.Getenv("CRON_SECRET")
		if expected == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+expected)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 240*time.Second)
	defer cancel()
	snapshot, err := h.database.latest(ctx)
	if err != nil {
		h.log.Error("load hosted snapshot", "err", err)
		http.Error(w, "The charts are temporarily unavailable. Please try again shortly.", http.StatusServiceUnavailable)
		return
	}
	store := NewStore()
	store.Load(snapshot)
	refresher := NewRefresher(store, h.database, h.fetcher, NewCompatClient(h.fetcher), h.interval, h.log)
	manual := r.Method == http.MethodPost && r.URL.Path == "/refresh"
	poll := r.Method == http.MethodGet && (r.URL.Path == "/boards" || (r.Header.Get("HX-Request") == "true" && (r.URL.Path == "/top-games" || r.URL.Path == "/trending")))
	due := snapshot.UpdatedAt.IsZero() || time.Since(snapshot.UpdatedAt) >= h.interval || len(snapshot.Errors) > 0
	if manual || cron || poll && due {
		window := h.interval
		if manual || len(snapshot.Errors) > 0 {
			window = 5 * time.Minute
		}
		retry, err := h.database.takeSlot(ctx, "hosted-refresh", time.Now(), window)
		if err != nil {
			h.log.Error("hosted refresh lease", "err", err)
			http.Error(w, "Couldn’t refresh. Please try again.", http.StatusServiceUnavailable)
			return
		}
		if retry == 0 {
			refresher.Refresh(ctx)
			// Persist partial compatibility progress even if the invocation budget ended.
			saveCtx, saveCancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
			if err := h.database.save(saveCtx, store.Get()); err != nil {
				h.log.Error("save hosted progress", "err", err)
			}
			saveCancel()
		}
		if manual {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			if retry > 0 {
				w.Header().Set("Retry-After", fmt.Sprint(int(retry.Seconds())+1))
				fmt.Fprintf(w, "Charts refresh automatically. Try again in %ds.", int(retry.Seconds())+1)
			} else {
				w.WriteHeader(http.StatusAccepted)
				fmt.Fprint(w, "Charts refreshed.")
			}
			return
		}
	}
	if cron {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
		return
	}
	app := *h.app
	app.store = store
	app.refresher = refresher
	app.Handler().ServeHTTP(w, r)
}
