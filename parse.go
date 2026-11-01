package main

import (
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

func parseTop(r io.Reader, limit int) ([]Game, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	games := make(map[int]Game)

	if sel := doc.Find("article.tc-throne").First(); sel.Length() > 0 {
		g := Game{
			Rank:  1,
			Title: cleanText(sel.Find(".tc-ttl").First().Text()),
			URL:   absURL(topGamesURL, sel.Find("a.lb-btn").First().AttrOr("href", "")),
			Image: imageURL(sel.Find(".tc-art img").First(), topGamesURL),
			Size:  cleanText(sel.Find(".tc-meta .m").First().Text()),
		}
		g.Genres, g.Year = splitGenres(sel.Find(".tc-gen").First().Text())
		d := sel.Find(".tc-dl .big").First()
		g.Downloads = intAttr(d, "data-count")
		if g.Downloads == 0 {
			g.Downloads = parseCompact(d.Text())
		}
		if fields := strings.Fields(cleanText(sel.Find(".tc-meta .tc-chip").First().Text())); len(fields) > 0 {
			g.Week = parseCompact(fields[0])
		}
		if g.Title != "" {
			games[g.Rank] = g
		}
	}

	doc.Find("article.tc-sec").Each(func(_ int, sel *goquery.Selection) {
		rank, _ := strconv.Atoi(cleanText(sel.Find(".rk").First().Text()))
		if rank == 0 {
			return
		}
		g := Game{
			Rank:      rank,
			Title:     cleanText(sel.Find(".t").First().Text()),
			URL:       absURL(topGamesURL, sel.Find(".in").First().AttrOr("href", "")),
			Image:     imageURL(sel.Find(".a img").First(), topGamesURL),
			Downloads: parseCompact(sel.Find(".d .big").First().Text()),
		}
		g.Genres, g.Year = splitGenres(sel.Find(".g").First().Text())
		if g.Title != "" {
			games[rank] = g
		}
	})

	doc.Find("div.lb-row[id^='rank-']").Each(func(_ int, sel *goquery.Selection) {
		rank, _ := strconv.Atoi(sel.AttrOr("data-rank", ""))
		if rank == 0 {
			return
		}
		titleLink := sel.Find(".lb-main .ct").First()
		title := cleanText(titleLink.Text())
		if title == "" {
			return
		}
		g := Game{
			Rank:      rank,
			Title:     title,
			URL:       absURL(topGamesURL, titleLink.AttrOr("href", "")),
			Image:     imageURL(sel.Find(".lb-thumb img").First(), topGamesURL),
			Downloads: intAttr(sel, "data-dl"),
			Week:      intAttr(sel, "data-wk"),
			Size:      cleanText(sel.Find(".lb-csz").Contents().First().Text()),
			Move:      cleanText(sel.Find(".lb-move").First().Text()),
		}
		g.Genres, g.Year = splitGenres(sel.Find(".lb-main .cg").First().Text())
		if year, err := strconv.Atoi(sel.AttrOr("data-year", "")); err == nil {
			g.Year = year
		}
		if g.Move == "•" {
			g.Move = ""
		}
		games[rank] = g
	})

	out := collectGames(games, limit)
	if len(out) == 0 {
		return nil, fmt.Errorf("no top games found")
	}
	return out, nil
}

func parseTrending(r io.Reader, limit int) ([]Game, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}

	games := make(map[int]Game)

	if sel := doc.Find("a.tw-throne").First(); sel.Length() > 0 {
		g := Game{
			Rank:      1,
			Title:     cleanText(sel.Find(".in h3").First().Text()),
			URL:       absURL(trendingURL, sel.AttrOr("href", "")),
			Image:     imageURL(sel.Find(".bg img").First(), trendingURL),
			Downloads: parseIntText(sel.Find(".st b").First().Text()),
			Peak:      "No.1",
			Move:      "No.1",
			MoveClass: "crown",
		}
		if g.Title != "" {
			games[1] = g
		}
	}

	doc.Find("a.tw-row").Each(func(_ int, sel *goquery.Selection) {
		h3 := sel.Find("h3").First()
		h3.Find("span.tw-steamy").Remove()
		title := cleanText(h3.Text())
		if title == "" {
			return
		}
		rank, _ := strconv.Atoi(cleanText(sel.Find(".tw-tr .to").First().Text()))
		if rank == 0 {
			return
		}
		g := Game{
			Rank:      rank,
			Title:     title,
			URL:       absURL(trendingURL, sel.AttrOr("href", "")),
			Image:     imageURL(sel.Find("img").First(), trendingURL),
			Downloads: parseIntText(sel.Find(".tw-qty .n").First().Text()),
			Move:      cleanText(sel.Find(".tw-gain").First().Text()),
		}
		g.Genres, _ = splitGenres(sel.Find(".gn").First().Text())

		gain := sel.Find(".tw-gain").First()
		switch {
		case gain.HasClass("added"):
			g.MoveClass = "new"
		case gain.HasClass("climbing"):
			g.MoveClass = "up"
		case gain.HasClass("slipping"):
			g.MoveClass = "down"
		}

		sel.Find(".tw-meta span").Each(func(_ int, s *goquery.Selection) {
			text := cleanText(s.Text())
			value := cleanText(s.Find("b").First().Text())
			switch {
			case strings.HasPrefix(text, "Weeks"):
				g.Weeks, _ = strconv.Atoi(value)
			case strings.HasPrefix(text, "Peak"):
				g.Peak = value
			}
		})

		games[rank] = g
	})

	out := collectGames(games, limit)
	if len(out) == 0 {
		return nil, fmt.Errorf("no trending games found")
	}
	return out, nil
}

func collectGames(games map[int]Game, limit int) []Game {
	ordered := make([]Game, 0, len(games))
	for _, g := range games {
		ordered = append(ordered, g)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Rank < ordered[j].Rank })
	out := make([]Game, 0, len(games))
	seen := make(map[string]bool, len(games))
	for _, g := range ordered {
		if g.Rank <= 0 || g.Title == "" {
			continue
		}
		key := g.URL
		if key == "" {
			key = g.Title
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, g)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func splitGenres(raw string) ([]string, int) {
	raw = cleanText(raw)
	if raw == "" {
		return nil, 0
	}

	var parts []string
	switch {
	case strings.Contains(raw, "·"):
		parts = strings.Split(raw, "·")
	case strings.Contains(raw, "/"):
		parts = strings.Split(raw, "/")
	default:
		parts = []string{raw}
	}

	var genres []string
	var year int
	for _, part := range parts {
		part = cleanText(part)
		if part == "" {
			continue
		}
		if y, err := strconv.Atoi(part); err == nil && y >= 1900 && y <= 2200 {
			year = y
			continue
		}
		genres = append(genres, part)
	}
	return genres, year
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u200b", "")
	return strings.Join(strings.Fields(s), " ")
}

func absURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if u.IsAbs() {
		return u.String()
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return b.ResolveReference(u).String()
}

func imageURL(sel *goquery.Selection, base string) string {
	if sel.Length() == 0 {
		return ""
	}
	src := sel.AttrOr("src", "")
	if src == "" {
		src = sel.AttrOr("data-src", "")
	}
	return absURL(base, src)
}

func intAttr(sel *goquery.Selection, name string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(sel.AttrOr(name, "")), 10, 64)
	return n
}

func parseCompact(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ",", "")

	multiplier := int64(1)
	switch {
	case strings.HasSuffix(s, "B"):
		multiplier = 1_000_000_000
		s = strings.TrimSuffix(s, "B")
	case strings.HasSuffix(s, "M"):
		multiplier = 1_000_000
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "K"):
		multiplier = 1_000
		s = strings.TrimSuffix(s, "K")
	}

	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int64(f * float64(multiplier))
}

func parseIntText(s string) int64 {
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
	n, _ := strconv.ParseInt(digits, 10, 64)
	return n
}
