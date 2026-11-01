package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	topGamesURL = "https://ankergames.net/top-games"
	trendingURL = "https://ankergames.net/trending"
	browserUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	maxPageSize = 8 << 20
)

type Fetcher struct {
	client   *http.Client
	jinaBase string
	jinaKey  string
}

func NewFetcher(jinaKey string) *Fetcher {
	return &Fetcher{
		client:   &http.Client{Timeout: 45 * time.Second},
		jinaBase: "https://r.jina.ai/",
		jinaKey:  jinaKey,
	}
}

// Get tries the source directly first. AnkerGames sits behind a Cloudflare
// managed challenge, so a 403 is expected; the Jina Reader then fetches the
// same page with a real browser and returns its HTML.
func (f *Fetcher) Get(ctx context.Context, rawURL string) ([]byte, error) {
	directCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	body, directErr := f.direct(directCtx, rawURL)
	cancel()
	if directErr == nil && !isChallenge(body) {
		return body, nil
	}

	body, readerErr := f.reader(ctx, rawURL)
	if readerErr == nil && !isChallenge(body) {
		return body, nil
	}

	if readerErr == nil {
		readerErr = fmt.Errorf("reader returned a challenge")
	}
	if directErr != nil {
		return nil, fmt.Errorf("direct: %w; reader: %v", directErr, readerErr)
	}
	return nil, readerErr
}

func (f *Fetcher) direct(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return body, nil
}

func (f *Fetcher) reader(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.jinaBase+rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("X-Return-Format", "html")
	if f.jinaKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.jinaKey)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, firstLine(body))
	}
	if !bytes.Contains(body, []byte("<html")) {
		return nil, fmt.Errorf("reader returned non-HTML content")
	}
	return body, nil
}

func (f *Fetcher) Top(ctx context.Context) ([]Game, error) {
	var games []Game
	pageURL := topGamesURL
	visited := make(map[string]bool)
	for page := 0; page < 10 && pageURL != "" && len(games) < 200; page++ {
		if visited[pageURL] {
			return nil, fmt.Errorf("top chart pagination repeated %s", pageURL)
		}
		visited[pageURL] = true
		body, err := f.Get(ctx, pageURL)
		if err != nil {
			return nil, fmt.Errorf("top chart page %d: %w", page+1, err)
		}
		pageGames, err := parseTop(bytes.NewReader(body), 0)
		if err != nil {
			return nil, fmt.Errorf("top chart page %d: %w", page+1, err)
		}
		games = append(games, pageGames...)
		pageURL = topNextURL(body)
	}
	return collectUniqueGames(games, 200), nil
}

func (f *Fetcher) Trending(ctx context.Context) ([]Game, error) {
	body, err := f.Get(ctx, trendingURL)
	if err != nil {
		return nil, err
	}
	return parseTrending(bytes.NewReader(body), 200)
}

func topNextURL(body []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	pageURL := absURL(topGamesURL, doc.Find("#lbPager").AttrOr("data-next", ""))
	u, err := url.Parse(pageURL)
	if err != nil || u.Scheme != "https" || u.Host != "ankergames.net" {
		return ""
	}
	return u.String()
}

func collectUniqueGames(games []Game, limit int) []Game {
	byRank := make(map[int]Game, len(games))
	for _, game := range games {
		if game.Rank > 0 && game.Title != "" {
			byRank[game.Rank] = game
		}
	}
	return collectGames(byRank, limit)
}

func isChallenge(body []byte) bool {
	return bytes.Contains(body, []byte("Just a moment...")) ||
		bytes.Contains(body, []byte("cf_chl_opt")) ||
		bytes.Contains(body, []byte("challenge-platform"))
}

func firstLine(body []byte) string {
	line := strings.TrimSpace(string(body))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if len(line) > 160 {
		line = line[:160]
	}
	return line
}
