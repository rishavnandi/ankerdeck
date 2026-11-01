package main

import (
	"sync"
	"time"
)

type Game struct {
	Rank      int
	Title     string
	URL       string
	Image     string
	Genres    []string
	Year      int
	Size      string
	Downloads int64
	Week      int64
	Move      string
	MoveClass string
	Weeks     int
	Peak      string
	Compat    Compat
}

type Compat struct {
	Status       string
	Label        string
	Stars        int
	MatchName    string
	URL          string
	Tooltip      string
	Checked      bool
	CheckedAt    time.Time
	MatchVersion int
}

type Snapshot struct {
	Top               []Game
	Trending          []Game
	UpdatedAt         time.Time
	TopUpdatedAt      time.Time
	TrendingUpdatedAt time.Time
	Errors            []string
	CompatDone        int
	CompatTotal       int
	CompatBusy        bool
}

type Store struct {
	mu   sync.RWMutex
	snap Snapshot
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Load(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot.Top = carryCompat(snapshot.Top, nil)
	snapshot.Trending = carryCompat(snapshot.Trending, nil)
	snapshot.CompatBusy = false
	s.snap = snapshot
}

func (s *Store) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

func (s *Store) SetBoards(top, trending []Game, errs []string, updated bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := make(map[string]Compat)
	for _, list := range [][]Game{s.snap.Top, s.snap.Trending} {
		for _, g := range list {
			if g.Compat.Checked && g.Compat.MatchVersion == compatMatchVersion {
				previous[normalizeTitle(g.Title)] = g.Compat
			}
		}
	}

	// Older snapshots have one timestamp; preserve it before a partial update.
	if s.snap.TopUpdatedAt.IsZero() && len(s.snap.Top) > 0 {
		s.snap.TopUpdatedAt = s.snap.UpdatedAt
	}
	if s.snap.TrendingUpdatedAt.IsZero() && len(s.snap.Trending) > 0 {
		s.snap.TrendingUpdatedAt = s.snap.UpdatedAt
	}
	s.snap.Top = carryCompat(top, previous)
	s.snap.Trending = carryCompat(trending, previous)
	s.snap.Errors = append([]string(nil), errs...)
	if updated {
		s.snap.UpdatedAt = time.Now()
	}
}

func carryCompat(games []Game, previous map[string]Compat) []Game {
	out := make([]Game, len(games))
	copy(out, games)
	for i := range out {
		if c, ok := previous[normalizeTitle(out[i].Title)]; ok {
			out[i].Compat = c
			continue
		}
		if out[i].Compat.Status == "" || out[i].Compat.MatchVersion != compatMatchVersion {
			out[i].Compat = pendingCompat(out[i].Title)
		}
	}
	return out
}

func (s *Store) SetCompat(title string, c Compat) {
	key := normalizeTitle(title)

	s.mu.Lock()
	defer s.mu.Unlock()

	update := func(games []Game) []Game {
		out := make([]Game, len(games))
		copy(out, games)
		for i := range out {
			if normalizeTitle(out[i].Title) != key {
				continue
			}
			// A transient lookup failure must not replace a good cached result.
			if c.Status == "error" && out[i].Compat.Checked && out[i].Compat.Status != "error" {
				continue
			}
			out[i].Compat = c
		}
		return out
	}

	s.snap.Top = update(s.snap.Top)
	s.snap.Trending = update(s.snap.Trending)
}

func (s *Store) SetProgress(done, total int, busy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.CompatDone = done
	s.snap.CompatTotal = total
	s.snap.CompatBusy = busy
}

func pendingCompat(title string) Compat {
	return Compat{
		Status:  "pending",
		Label:   "Checking",
		URL:     compatSearchURL(title),
		Tooltip: "Checking CodeWeavers for a CrossOver report.",
	}
}

func (s *Store) SetBoardTimes(topOK, trendingOK bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if topOK {
		s.snap.TopUpdatedAt = now
	}
	if trendingOK {
		s.snap.TrendingUpdatedAt = now
	}
}
