package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHostedPagesUsePersistentSnapshotAndProtectCron(t *testing.T) {
	database, err := openSnapshotDB(filepath.Join(t.TempDir(), "hosted.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.close()
	snapshot := Snapshot{Top: []Game{{Rank: 1, Title: "Persistent game"}}, Trending: []Game{{Rank: 1, Title: "Trending game"}}, UpdatedAt: time.Now()}
	if err := database.save(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(NewStore(), 15*time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	fetcher := NewFetcher("")
	fetcher.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("fresh chart should not fetch upstream")
		return htmlResponse(""), nil
	})
	handler := &hostedHandler{app: app, database: database, fetcher: fetcher, log: slog.New(slog.NewTextHandler(io.Discard, nil)), interval: 15 * time.Minute}
	for _, path := range []string{"/", "/top-games", "/trending", "/boards"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "game") {
			t.Fatalf("%s did not restore stored games: status=%d", path, response.Code)
		}
	}
	t.Setenv("CRON_SECRET", "test-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/cron", nil))
	if response.Code != 401 {
		t.Fatalf("unauthenticated cron status=%d", response.Code)
	}
}

func TestRefreshLeaseHasOneWinner(t *testing.T) {
	database, err := openSnapshotDB(filepath.Join(t.TempDir(), "lease.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.close()
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			retry, err := database.takeSlot(context.Background(), "concurrent", time.Now(), time.Minute)
			if err != nil {
				t.Error(err)
				return
			}
			if retry == 0 {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("lease winners=%d", winners.Load())
	}
}
