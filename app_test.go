package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHomePreviewAndFullListRoutes(t *testing.T) {
	store := NewStore()
	games := make([]Game, 6)
	for i := range games {
		games[i] = Game{Rank: i + 1, Title: fmt.Sprintf("Fixture Game %d", i+1), URL: "https://ankergames.net/game"}
	}
	store.Load(Snapshot{Top: games, Trending: games, UpdatedAt: time.Now()})
	app, err := NewApp(store, 15*time.Minute, &Refresher{})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Handler()

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	if home.Code != 200 || strings.Count(home.Body.String(), `class="game-row"`) != 10 || strings.Contains(home.Body.String(), "Fixture Game 6") {
		t.Fatalf("home preview status=%d rows=%d includes sixth=%t", home.Code, strings.Count(home.Body.String(), `class="game-row"`), strings.Contains(home.Body.String(), "Fixture Game 6"))
	}

	for _, path := range []string{"/top-games", "/trending"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), "Fixture Game 6") {
			t.Errorf("%s status=%d full list missing sixth title", path, response.Code)
		}
	}
}

func TestDarkNavbarUsesThemeCanvas(t *testing.T) {
	app, err := NewApp(NewStore(), 15*time.Minute, &Refresher{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/static/app.css", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), ".site-header {") || !strings.Contains(response.Body.String(), "background: var(--canvas);") {
		t.Fatalf("stylesheet does not use the theme canvas")
	}
	page := httptest.NewRecorder()
	app.Handler().ServeHTTP(page, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(page.Body.String(), "/static/app.css?v=4") {
		t.Fatal("home page does not bust the stale stylesheet cache")
	}
}

func TestListPollingReturnsOnlyResults(t *testing.T) {
	store := NewStore()
	store.Load(Snapshot{Top: []Game{{Rank: 1, Title: "A game"}}, UpdatedAt: time.Now(), CompatBusy: true})
	app, err := NewApp(store, 15*time.Minute, &Refresher{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/top-games", nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != 200 || !strings.Contains(body, `id="list-results"`) || strings.Contains(body, "<html") || strings.Contains(body, `id="game-search"`) {
		t.Fatalf("unexpected fragment: %s", body)
	}
	if response.Header().Get("Vary") != "HX-Request" {
		t.Fatal("missing fragment cache variation")
	}
}
