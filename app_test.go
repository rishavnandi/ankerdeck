package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHomePreviewAndFullListRoutesUseSavedSnapshot(t *testing.T) {
	games := make([]Game, 6)
	for i := range games {
		games[i] = Game{Rank: i + 1, Title: fmt.Sprintf("Fixture Game %d", i+1), URL: "https://ankergames.net/game"}
	}
	snapshot := Snapshot{Top: games, Trending: games, UpdatedAt: time.Now()}
	app, err := NewApp(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Handler()

	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequest("GET", "/", nil))
	if home.Code != http.StatusOK || strings.Count(home.Body.String(), `class="game-row"`) != 10 || strings.Contains(home.Body.String(), "Fixture Game 6") {
		t.Fatalf("home preview status=%d rows=%d includes sixth=%t", home.Code, strings.Count(home.Body.String(), `class="game-row"`), strings.Contains(home.Body.String(), "Fixture Game 6"))
	}
	if strings.Contains(home.Body.String(), "hx-") || strings.Contains(home.Body.String(), "Refresh") {
		t.Fatal("home page still contains automatic or manual refresh controls")
	}

	for _, path := range []string{"/top-games", "/trending"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Fixture Game 6") {
			t.Errorf("%s status=%d full list missing sixth title", path, response.Code)
		}
	}
}

func TestLegacyErrorsDoNotRenderWarning(t *testing.T) {
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(`{"Top":[{"Rank":1,"Title":"Saved game"}],"Errors":["top games: reader returned a challenge"]}`), &snapshot); err != nil {
		t.Fatal(err)
	}
	app, err := NewApp(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "Saved game") {
		t.Fatalf("saved snapshot did not render: status=%d", response.Code)
	}
	if strings.Contains(body, "couldn’t be updated") || strings.Contains(body, "Retrying") || strings.Contains(body, "reader returned a challenge") {
		t.Fatal("legacy refresh error still appears in the page")
	}
}

func TestUpdaterRoutesAreRemoved(t *testing.T) {
	app, err := NewApp(Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/refresh"},
		{http.MethodPost, "/refresh"},
		{http.MethodGet, "/boards"},
		{http.MethodGet, "/api/cron"},
	} {
		response := httptest.NewRecorder()
		app.Handler().ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s status=%d; want 404", request.method, request.path, response.Code)
		}
	}
}

func TestStaticAssetsAndThemeCacheVersion(t *testing.T) {
	app, err := NewApp(Snapshot{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/static/app.css", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), ".site-header {") || !strings.Contains(response.Body.String(), "background: var(--canvas);") {
		t.Fatalf("stylesheet does not use the theme canvas")
	}
	page := httptest.NewRecorder()
	app.Handler().ServeHTTP(page, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(page.Body.String(), "/static/app.css?v=5") {
		t.Fatal("home page does not bust the stylesheet cache")
	}
}

func TestListDoesNotReturnPollingFragment(t *testing.T) {
	app, err := NewApp(Snapshot{Top: []Game{{Rank: 1, Title: "A game"}}, UpdatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/top-games", nil)
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	app.Handler().ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `id="game-search"`) || !strings.Contains(body, "<html") {
		t.Fatalf("request did not receive the full saved page: %s", body)
	}
	if response.Header().Get("Vary") != "" {
		t.Fatal("removed polling endpoint still varies on HX-Request")
	}
}
