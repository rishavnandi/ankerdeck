package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRefresherRestoresCacheWithoutExtendingExpiry(t *testing.T) {
	checked := time.Now().Add(-23 * time.Hour)
	store := NewStore()
	store.Load(Snapshot{Top: []Game{{Title: "Fresh", Compat: Compat{Checked: true, CheckedAt: checked, MatchVersion: compatMatchVersion, Status: "great"}}, {Title: "Expired", Compat: Compat{Checked: true, CheckedAt: time.Now().Add(-25 * time.Hour), MatchVersion: compatMatchVersion, Status: "good"}}, {Title: "Old matcher", Compat: Compat{Checked: true, CheckedAt: time.Now(), MatchVersion: compatMatchVersion - 1, Status: "good"}}}})
	r := NewRefresher(store, nil, nil, nil, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if c, ok := r.cache.get("fresh"); !ok || c.Status != "great" {
		t.Fatal("fresh cache was not restored")
	}
	if !r.cache.entries["fresh"].at.Equal(checked) {
		t.Fatal("cache expiry was extended")
	}
	if _, ok := r.cache.get("expired"); ok {
		t.Fatal("restored an expired result")
	}
	if _, ok := r.cache.get("old matcher"); ok {
		t.Fatal("restored a superseded match")
	}
}

func TestCanceledEnrichmentDoesNotClaimEverythingChecked(t *testing.T) {
	store := NewStore()
	store.Load(Snapshot{Top: []Game{{Title: "One"}, {Title: "Two"}}})
	r := NewRefresher(store, nil, nil, &CompatClient{fetch: func(ctx context.Context, _ string) ([]byte, error) { return nil, ctx.Err() }}, time.Minute, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.enrichCompat(ctx)
	got := store.Get()
	if got.CompatBusy || got.CompatDone != 0 || got.CompatTotal != 2 {
		t.Fatalf("misleading progress: %+v", got)
	}
}

func TestCompatibilityFailureKeepsKnownRating(t *testing.T) {
	store := NewStore()
	store.Load(Snapshot{Top: []Game{{Title: "One", Compat: Compat{Checked: true, CheckedAt: time.Now(), MatchVersion: compatMatchVersion, Status: "great", Stars: 5}}}})
	store.SetCompat("One", Compat{Status: "error"})
	if got := store.Get().Top[0].Compat; got.Stars != 5 || got.Status != "great" {
		t.Fatalf("transient failure erased rating: %+v", got)
	}
}

func TestPartialRefreshPreservesOtherChartTimestamp(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	store := NewStore()
	store.Load(Snapshot{Top: []Game{{Title: "Top"}}, Trending: []Game{{Title: "Trending"}}, UpdatedAt: old})
	store.SetBoards(store.Get().Top, []Game{{Title: "New trending"}}, []string{"top games: unavailable"}, true)
	store.SetBoardTimes(false, true)
	got := store.Get()
	if !got.TopUpdatedAt.Equal(old) || !got.TrendingUpdatedAt.After(old) {
		t.Fatalf("partial refresh lost chart freshness: top=%v trending=%v", got.TopUpdatedAt, got.TrendingUpdatedAt)
	}
}
