package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type App struct {
	store     *Store
	refresher *Refresher
	interval  time.Duration
	templates *template.Template
	static    http.Handler
}

func NewApp(store *Store, interval time.Duration, refresher *Refresher) (*App, error) {
	t, err := template.New("").Funcs(template.FuncMap{
		"num":          formatNum,
		"ago":          ago,
		"genres":       genreLabel,
		"initial":      initial,
		"duration":     humanDuration,
		"take":         takeGames,
		"ratingLegend": ratingLegend,
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}

	return &App{
		store:     store,
		refresher: refresher,
		interval:  interval,
		templates: t,
		static:    http.FileServer(http.FS(sub)),
	}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.index)
	mux.HandleFunc("GET /boards", a.boards)
	mux.HandleFunc("GET /top-games", func(w http.ResponseWriter, r *http.Request) { a.list(w, r, "top") })
	mux.HandleFunc("GET /trending", func(w http.ResponseWriter, r *http.Request) { a.list(w, r, "trending") })
	mux.HandleFunc("POST /refresh", a.refresh)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		a.static.ServeHTTP(w, r)
	})))

	return securityHeaders(mux)
}

func (a *App) refresh(w http.ResponseWriter, r *http.Request) {
	retry, err := a.refresher.RequestManual(r.Context())
	if err != nil {
		slog.Error("manual refresh", "err", err)
		http.Error(w, "Could not queue refresh.", http.StatusInternalServerError)
		return
	}
	if retry > 0 {
		seconds := int(retry.Seconds()) + 1
		w.Header().Set("Retry-After", fmt.Sprint(seconds))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "Charts refresh automatically. Try again in %ds.", seconds)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("Updating the charts…"))
}

func (a *App) index(w http.ResponseWriter, _ *http.Request) {
	a.render(w, "index.html", a.pageData(true))
}

func (a *App) boards(w http.ResponseWriter, _ *http.Request) {
	a.render(w, "boards.html", a.pageData(false))
}

func (a *App) list(w http.ResponseWriter, r *http.Request, kind string) {
	data := a.pageData(false)
	data.ListKind = kind
	if kind == "top" {
		data.ListTitle, data.Games = "Top games", data.Snapshot.Top
	} else {
		data.ListTitle, data.Games = "Trending", data.Snapshot.Trending
	}
	errs := data.Snapshot.Errors
	data.Snapshot.Errors = nil
	prefix := "top games:"
	if kind == "trending" {
		prefix = "trending:"
	}
	for _, errText := range errs {
		if strings.HasPrefix(errText, prefix) {
			data.Snapshot.Errors = append(data.Snapshot.Errors, errText)
		}
	}
	w.Header().Set("Vary", "HX-Request")
	if r.Header.Get("HX-Request") == "true" {
		a.render(w, "list-results", data)
		return
	}
	a.render(w, "list.html", data)
}

func (a *App) pageData(initial bool) PageData {
	return PageData{
		Snapshot:     a.store.Get(),
		RefreshEvery: humanDuration(a.interval),
		Initial:      initial,
	}
}

func (a *App) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := a.templates.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("render template", "template", name, "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

type PageData struct {
	Snapshot     Snapshot
	RefreshEvery string
	Initial      bool
	ListKind     string
	ListTitle    string
	Games        []Game
}

func takeGames(games []Game, count int) []Game {
	if len(games) > count {
		return games[:count]
	}
	return games
}

func (p PageData) Loading() bool {
	if p.ListKind == "top" {
		return len(p.Snapshot.Top) == 0
	}
	if p.ListKind == "trending" {
		return len(p.Snapshot.Trending) == 0
	}
	return p.Snapshot.UpdatedAt.IsZero()
}
func (p PageData) Busy() bool { return p.Snapshot.CompatBusy }

func (p PageData) StatusClass() string {
	switch {
	case p.Loading():
		return "loading"
	case len(p.Snapshot.Errors) > 0:
		return "warn"
	default:
		return "live"
	}
}

func (p PageData) StatusText() string {
	switch {
	case p.Loading():
		return "Loading"
	case len(p.Snapshot.Errors) > 0:
		return "Retrying"
	default:
		return "Live"
	}
}

func (p PageData) UpdatedText() string {
	if p.Loading() {
		return "Loading charts…"
	}
	updated := p.Snapshot.UpdatedAt
	if p.ListKind == "top" && !p.Snapshot.TopUpdatedAt.IsZero() {
		updated = p.Snapshot.TopUpdatedAt
	}
	if p.ListKind == "trending" && !p.Snapshot.TrendingUpdatedAt.IsZero() {
		updated = p.Snapshot.TrendingUpdatedAt
	}
	return "Updated " + ago(updated)
}

func (p PageData) CompatText() string {
	switch {
	case p.Snapshot.CompatTotal == 0:
		return "Compatibility queued"
	case p.Snapshot.CompatBusy:
		return fmt.Sprintf("Checking compatibility %d/%d", p.Snapshot.CompatDone, p.Snapshot.CompatTotal)
	default:
		return "Compatibility checked"
	}
}

// Trigger is the htmx poll spec. The full page may request an immediate load;
// a fragment must not, or every swap would trigger another request in a loop.
func (p PageData) Trigger() string {
	switch {
	case p.Loading():
		if p.Initial {
			return "load, every 5s"
		}
		return "every 5s"
	case p.Busy():
		return "every 10s"
	default:
		return "every 30s"
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' https://ankergames.net data:; style-src 'self' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'self'; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func ratingLegend() []Compat {
	var out []Compat
	for stars := 5; stars >= 0; stars-- {
		status, label := compatStatus(stars)
		out = append(out, Compat{Status: status, Label: label, Stars: stars})
	}
	return append(out, Compat{Status: "none", Label: "No report"})
}
