package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// hostedHandler reloads the saved snapshot for every page request so separate
// Vercel invocations always render the latest manually imported charts.
type hostedHandler struct {
	app      *App
	database *snapshotDB
	log      *slog.Logger
}

func (h *hostedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/static/") {
		h.app.Handler().ServeHTTP(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	snapshot, err := h.database.latest(ctx)
	if err != nil {
		h.log.Error("load hosted snapshot", "err", err)
		http.Error(w, "The charts are temporarily unavailable. Please try again shortly.", http.StatusServiceUnavailable)
		return
	}

	app := *h.app
	app.snapshot = snapshot
	app.Handler().ServeHTTP(w, r)
}
