package main

import "time"

// Game is one ranked entry in a manually imported AnkerGames chart snapshot.
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

// Compat contains the static rating fields used to render a game's badge.
// The extra metadata remains in the JSON model so snapshots created by older
// versions can be imported without altering their compatibility records.
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

// Snapshot is one complete manually imported version of both charts.
type Snapshot struct {
	Top               []Game
	Trending          []Game
	UpdatedAt         time.Time
	TopUpdatedAt      time.Time
	TrendingUpdatedAt time.Time
}
