package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotDBRoundTrip(t *testing.T) {
	db, err := openSnapshotDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.close() })

	want := Snapshot{
		Top:               []Game{{Rank: 1, Title: "Test game", Compat: Compat{Status: "good", Label: "Runs well", Stars: 4, Checked: true}}},
		Trending:          []Game{{Rank: 1, Title: "Trending game"}},
		UpdatedAt:         time.Now().UTC(),
		TopUpdatedAt:      time.Now().UTC(),
		TrendingUpdatedAt: time.Now().UTC(),
	}
	if err := db.save(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := db.latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Top) != 1 || got.Top[0].Title != want.Top[0].Title || got.Top[0].Compat.Status != "good" {
		t.Fatalf("snapshot round trip = %+v", got)
	}
	if len(got.Trending) != 1 || got.Trending[0].Title != want.Trending[0].Title {
		t.Fatalf("trending snapshot round trip = %+v", got.Trending)
	}
}

func TestImportSnapshotWritesBothCharts(t *testing.T) {
	db, err := openSnapshotDB(filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.close() })

	want := Snapshot{
		Top:       []Game{{Rank: 1, Title: "Seeded top", Compat: Compat{Status: "great", Label: "Runs great", Stars: 5}}},
		Trending:  []Game{{Rank: 1, Title: "Seeded trend"}},
		UpdatedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	imported, err := importSnapshot(context.Background(), db, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Top) != 1 || len(imported.Trending) != 1 {
		t.Fatalf("imported counts top=%d trending=%d", len(imported.Top), len(imported.Trending))
	}
	got, err := db.latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Top[0].Title != "Seeded top" || got.Trending[0].Title != "Seeded trend" || got.Top[0].Compat.Stars != 5 {
		t.Fatalf("seeded database snapshot = %+v", got)
	}
}

func TestReadSnapshotRequiresBothCharts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.json")
	if err := os.WriteFile(path, []byte(`{"Top":[{"Title":"Only top"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSnapshot(path); err == nil {
		t.Fatal("accepted a seed missing its trending chart")
	}
}
