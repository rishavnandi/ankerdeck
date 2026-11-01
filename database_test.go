package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotDBRoundTripAndManualLimit(t *testing.T) {
	db, err := openSnapshotDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.close() })

	want := Snapshot{Top: []Game{{Rank: 1, Title: "Test game", Compat: Compat{Status: "good", Checked: true}}}, UpdatedAt: time.Now().UTC()}
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

	now := time.Now()
	if retry, err := db.takeManualSlot(context.Background(), now, 5*time.Minute); err != nil || retry != 0 {
		t.Fatalf("first slot = %v, %v", retry, err)
	}
	if retry, err := db.takeManualSlot(context.Background(), now.Add(time.Minute), 5*time.Minute); err != nil || retry != 4*time.Minute {
		t.Fatalf("second slot = %v, %v; want 4m", retry, err)
	}
}
