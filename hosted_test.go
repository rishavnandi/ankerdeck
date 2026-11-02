package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHostedPagesReloadLatestSnapshotPerRequest(t *testing.T) {
	database, err := openSnapshotDB(filepath.Join(t.TempDir(), "hosted.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.close()
	if err := database.save(context.Background(), Snapshot{
		Top:       []Game{{Rank: 1, Title: "First saved game"}},
		Trending:  []Game{{Rank: 1, Title: "First trending game"}},
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	handler := &hostedHandler{app: app, database: database, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	for _, path := range []string{"/", "/top-games", "/trending"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "First") {
			t.Fatalf("%s did not render the saved charts: status=%d", path, response.Code)
		}
	}

	if err := database.save(context.Background(), Snapshot{
		Top:       []Game{{Rank: 1, Title: "Latest saved game"}},
		Trending:  []Game{{Rank: 1, Title: "Latest trending game"}},
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Latest saved game") {
		t.Fatalf("page did not reload latest snapshot: status=%d", response.Code)
	}
}

func TestHostedSnapshotIgnoresLegacyErrorsAndHasNoUpdaterRoutes(t *testing.T) {
	database, err := openSnapshotDB(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.close()

	payload, err := json.Marshal(map[string]any{
		"Top":       []Game{{Rank: 1, Title: "Last saved chart"}},
		"Trending":  []Game{{Rank: 1, Title: "Saved trending chart"}},
		"UpdatedAt": time.Now(),
		"Errors":    []string{"top games: reader returned a challenge"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.db.ExecContext(context.Background(), `INSERT INTO runs(captured_at, payload) VALUES(?, ?)`, time.Now().UTC().Format(time.RFC3339Nano), payload); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	handler := &hostedHandler{app: app, database: database, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Last saved chart") {
		t.Fatalf("legacy snapshot status=%d, body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "couldn’t be updated") || strings.Contains(response.Body.String(), "Retrying") || strings.Contains(response.Body.String(), "reader returned a challenge") {
		t.Fatal("legacy refresh error still renders on the deployed page")
	}

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/cron"},
		{http.MethodGet, "/boards"},
		{http.MethodGet, "/refresh"},
		{http.MethodPost, "/refresh"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s status=%d; want 404", request.method, request.path, response.Code)
		}
	}
}
