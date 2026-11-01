package main

import (
	"os"
	"testing"
)

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestParseTop(t *testing.T) {
	games, err := parseTop(openFixture(t, "top.html"), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 20 {
		t.Fatalf("got %d games, want 20", len(games))
	}

	first := games[0]
	if first.Rank != 1 || first.Title != "Grand Theft Auto V" {
		t.Errorf("rank 1 = %d %q, want 1 Grand Theft Auto V", first.Rank, first.Title)
	}
	if first.Downloads != 2722840 {
		t.Errorf("rank 1 downloads = %d, want 2722840", first.Downloads)
	}
	if first.URL != "https://ankergames.net/game/grand-theft-auto-v" {
		t.Errorf("rank 1 URL = %q", first.URL)
	}

	var forza *Game
	for i := range games {
		if games[i].Title == "Forza Horizon 6" {
			forza = &games[i]
		}
	}
	if forza == nil {
		t.Fatal("Forza Horizon 6 missing")
	}
	if forza.Rank != 7 || forza.Year != 2026 || forza.Downloads != 649571 {
		t.Errorf("Forza Horizon 6 = rank %d year %d downloads %d", forza.Rank, forza.Year, forza.Downloads)
	}
	if len(forza.Genres) != 2 || forza.Genres[0] != "Simulation" || forza.Genres[1] != "Open World" {
		t.Errorf("Forza Horizon 6 genres = %v", forza.Genres)
	}
}

func TestParseTopLimit(t *testing.T) {
	games, err := parseTop(openFixture(t, "top.html"), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 3 {
		t.Fatalf("got %d games, want 3", len(games))
	}
	for i, g := range games {
		if g.Rank != i+1 {
			t.Errorf("games[%d].Rank = %d, want %d", i, g.Rank, i+1)
		}
	}
}

func TestParseTrending(t *testing.T) {
	games, err := parseTrending(openFixture(t, "trending.html"), 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 20 {
		t.Fatalf("got %d games, want 20", len(games))
	}

	first := games[0]
	if first.Rank != 1 || first.Title != "Call of Duty: Modern Warfare 4" {
		t.Errorf("rank 1 = %d %q", first.Rank, first.Title)
	}
	if first.Downloads != 63414 {
		t.Errorf("rank 1 downloads = %d, want 63414", first.Downloads)
	}

	second := games[1]
	if second.Title != "Grand Theft Auto V" || second.MoveClass != "up" || second.Move != "▲ 1" {
		t.Errorf("rank 2 = %q move %q class %q", second.Title, second.Move, second.MoveClass)
	}

	third := games[2]
	if third.Title != "Moon Sweepers" || third.MoveClass != "new" {
		t.Errorf("rank 3 = %q move class %q", third.Title, third.MoveClass)
	}

	var minecraft *Game
	for i := range games {
		if games[i].Title == "Minecraft" {
			minecraft = &games[i]
		}
	}
	if minecraft == nil {
		t.Fatal("Minecraft missing")
	}
	if minecraft.Weeks != 17 || minecraft.Peak != "No.2" || minecraft.Downloads != 11619 {
		t.Errorf("Minecraft = weeks %d peak %q downloads %d", minecraft.Weeks, minecraft.Peak, minecraft.Downloads)
	}
}

func TestParseTopRejectsEmptyPage(t *testing.T) {
	games, err := parseTop(openFixture(t, "compat-search.html"), 20)
	if err == nil || games != nil {
		t.Fatalf("got %v, %v; want nil, error", games, err)
	}
}

func TestTopNextURL(t *testing.T) {
	body, err := os.ReadFile("testdata/top.html")
	if err != nil {
		t.Fatal(err)
	}
	if next := topNextURL(body); next != "https://ankergames.net/top-games?page=2" {
		t.Fatalf("next page = %q", next)
	}
	if next := topNextURL([]byte(`<div id="lbPager" data-next="https://example.com/evil"></div>`)); next != "" {
		t.Fatalf("external next page = %q, want empty", next)
	}
}
