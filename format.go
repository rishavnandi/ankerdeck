package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func formatNum(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return trimZero(fmt.Sprintf("%.1fB", float64(n)/1_000_000_000))
	case n >= 1_000_000:
		return trimZero(fmt.Sprintf("%.1fM", float64(n)/1_000_000))
	case n >= 1_000:
		return trimZero(fmt.Sprintf("%.1fK", float64(n)/1_000))
	default:
		return strconv.FormatInt(n, 10)
	}
}

func trimZero(s string) string {
	s = strings.TrimSuffix(s, "0")
	s = strings.TrimSuffix(s, ".")
	return s
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	if d < 10*time.Second {
		return "just now"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func genreLabel(genres []string) string {
	if len(genres) > 2 {
		genres = genres[:2]
	}
	return strings.Join(genres, " · ")
}

func initial(s string) string {
	for _, r := range s {
		return strings.ToUpper(string(r))
	}
	return "?"
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}
