package main

import (
	"os"
	"testing"
)

func TestParseCompatResultsExactMatch(t *testing.T) {
	body, err := os.ReadFile("testdata/compat-search.html")
	if err != nil {
		t.Fatal(err)
	}
	got := parseCompatResults(body, "Minecraft", compatSearchURL("Minecraft"))

	if !got.Checked {
		t.Error("Checked = false, want true")
	}
	if got.MatchName != "Minecraft" {
		t.Errorf("MatchName = %q, want Minecraft", got.MatchName)
	}
	if got.Stars != 2 || got.Status != "issues" || got.Label != "Major issues" {
		t.Errorf("got %d stars, status %q, label %q", got.Stars, got.Status, got.Label)
	}
	wantURL := "https://www.codeweavers.com/compatibility/crossover/minecraft"
	if got.URL != wantURL {
		t.Errorf("URL = %q, want %q", got.URL, wantURL)
	}
}

func TestParseCompatResultsNoMatch(t *testing.T) {
	body, err := os.ReadFile("testdata/compat-search.html")
	if err != nil {
		t.Fatal(err)
	}
	got := parseCompatResults(body, "Definitely Not A Real Game", compatSearchURL("Definitely Not A Real Game"))

	if got.Status != "none" || got.Label != "No report" || got.Stars != 0 || got.MatchName != "" {
		t.Errorf("got status %q, label %q, stars %d, match %q", got.Status, got.Label, got.Stars, got.MatchName)
	}
	if !got.Checked {
		t.Error("Checked = false, want true")
	}
	if got.URL != compatSearchURL("Definitely Not A Real Game") {
		t.Errorf("URL = %q, want the CodeWeavers search URL", got.URL)
	}
}

func TestBestCompatPrefersExactTitle(t *testing.T) {
	candidates := []compatCandidate{
		{Name: "Minecraft Note Block Studio", Href: "/compatibility/crossover/minecraft-note-block-studio", Stars: 4},
		{Name: "Minecraft", Href: "/compatibility/crossover/minecraft", Stars: 2},
	}
	best, score := bestCompat("Minecraft", candidates)
	if best == nil || best.Name != "Minecraft" {
		t.Fatalf("best = %+v, want Minecraft", best)
	}
	if score != 1 {
		t.Errorf("score = %f, want 1", score)
	}
}

func TestBestCompatRejectsWeakMatch(t *testing.T) {
	candidates := []compatCandidate{{Name: "Grand Theft Auto IV", Stars: 4}}
	if best, _ := bestCompat("Grand Theft Auto V", candidates); best != nil {
		t.Errorf("best = %+v, want nil", best)
	}
}

func TestBestCompatRejectsDifferentModernWarfareTitle(t *testing.T) {
	candidates := []compatCandidate{{Name: "Call of Duty 4: Modern Warfare", Stars: 4}}
	if best, _ := bestCompat("Modern Warfare 4", candidates); best != nil {
		t.Errorf("best = %+v, want nil", best)
	}
}

func TestTitleScore(t *testing.T) {
	tests := []struct {
		query, candidate string
		want             float64
	}{
		{"Grand Theft Auto V", "Grand Theft Auto V", 1},
		{"Grand Theft Auto V", "Grand Theft Auto V Legacy Edition", 0.9333333333333333},
		{"Marvel’s Spider-Man Remastered", "Marvels Spider Man Remastered", 1},
		{"Far Cry 3", "Far Cry", 0.3666666666666667},
		{"Minecraft", "Minecraft Note Block Studio", 0.85},
		{"Minecraft", "Farming Simulator 25", 0},
		{"", "Minecraft", 0},
	}
	for _, tt := range tests {
		got := titleScore(tt.query, tt.candidate)
		if diff := got - tt.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("titleScore(%q, %q) = %v, want %v", tt.query, tt.candidate, got, tt.want)
		}
	}
}

func TestCompatStatus(t *testing.T) {
	tests := []struct {
		stars  int
		status string
		label  string
	}{
		{5, "great", "Runs great"},
		{4, "good", "Runs well"},
		{3, "playable", "Some issues"},
		{2, "issues", "Major issues"},
		{1, "broken", "Broken"},
		{0, "untested", "Untested"},
	}
	for _, tt := range tests {
		status, label := compatStatus(tt.stars)
		if status != tt.status || label != tt.label {
			t.Errorf("compatStatus(%d) = %q, %q; want %q, %q", tt.stars, status, label, tt.status, tt.label)
		}
	}
}

func TestMatchingKeepsGameVariantsSeparate(t *testing.T) {
	tests := []struct {
		query, candidate string
		match            bool
	}{
		{"Grand Theft Auto V", "Grand Theft Auto V Enhanced", false},
		{"Grand Theft Auto V", "Grand Theft Auto V Legacy Edition", true},
		{"Cyberpunk 2077 Ultimate Edition", "Cyberpunk 2077", true},
		{"God of War III Remastered", "God of War III", false},
		{"Minecraft", "Minecraft Launcher", false},
		{"Doom", "Doom Eternal", false},
		{"Call of Duty Modern Warfare 4", "Call of Duty 4 Modern Warfare", false},
	}
	for _, tt := range tests {
		t.Run(tt.query+"/"+tt.candidate, func(t *testing.T) {
			got, _ := bestCompat(tt.query, []compatCandidate{{Name: tt.candidate}})
			if (got != nil) != tt.match {
				t.Fatalf("match = %+v; want match %v", got, tt.match)
			}
		})
	}
}
