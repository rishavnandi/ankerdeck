package main

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type App struct {
	snapshot  Snapshot
	templates *template.Template
	static    http.Handler
}

func NewApp(snapshot Snapshot) (*App, error) {
	t, err := template.New("").Funcs(template.FuncMap{
		"num":          formatNum,
		"ago":          ago,
		"genres":       genreLabel,
		"initial":      initial,
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
		snapshot:  snapshot,
		templates: t,
		static:    http.FileServer(http.FS(sub)),
	}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.index)
	mux.HandleFunc("GET /top-games", func(w http.ResponseWriter, _ *http.Request) { a.list(w, "top") })
	mux.HandleFunc("GET /trending", func(w http.ResponseWriter, _ *http.Request) { a.list(w, "trending") })
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

func (a *App) index(w http.ResponseWriter, _ *http.Request) {
	a.render(w, "index.html", a.pageData())
}

func (a *App) list(w http.ResponseWriter, kind string) {
	data := a.pageData()
	data.ListKind = kind
	if kind == "top" {
		data.ListTitle, data.Games = "Top games", data.Snapshot.Top
	} else {
		data.ListTitle, data.Games = "Trending", data.Snapshot.Trending
	}
	a.render(w, "list.html", data)
}

func (a *App) pageData() PageData {
	return PageData{
		Snapshot: a.snapshot,
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
	Snapshot  Snapshot
	ListKind  string
	ListTitle string
	Games     []Game
}

func takeGames(games []Game, count int) []Game {
	if len(games) > count {
		return games[:count]
	}
	return games
}

func (p PageData) UpdatedText() string {
	updated := p.Snapshot.UpdatedAt
	if p.ListKind == "top" && !p.Snapshot.TopUpdatedAt.IsZero() {
		updated = p.Snapshot.TopUpdatedAt
	}
	if p.ListKind == "trending" && !p.Snapshot.TrendingUpdatedAt.IsZero() {
		updated = p.Snapshot.TrendingUpdatedAt
	}
	if updated.IsZero() {
		return "No saved chart data"
	}
	return "Updated " + ago(updated)
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
