package main

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const codeWeaversBase = "https://www.codeweavers.com"

const compatMatchVersion = 3

type CompatClient struct {
	fetch func(context.Context, string) ([]byte, error)
}

func NewCompatClient(f *Fetcher) *CompatClient {
	return &CompatClient{fetch: f.Get}
}

func compatSearchURL(title string) string {
	return codeWeaversBase + "/compatibility?name=" + url.QueryEscape(title)
}

func (c *CompatClient) Lookup(ctx context.Context, title string) (Compat, error) {
	result, err := c.lookup(ctx, title, title)
	base := baseEditionTitle(title)
	if err == nil && result.Status == "none" && base != normalizeTitle(title) {
		return c.lookup(ctx, title, base)
	}
	return result, err
}

func (c *CompatClient) lookup(ctx context.Context, title, query string) (Compat, error) {
	searchURL := compatSearchURL(query)
	body, err := c.fetch(ctx, searchURL)
	if err != nil {
		return Compat{}, err
	}
	if isChallenge(body) {
		return Compat{}, fmt.Errorf("codeweavers returned a challenge")
	}
	// A changed source page must not become a cached false "No report".
	doc, parseErr := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if parseErr != nil || (doc.Find("tr[id^='key_']").Length() == 0 && doc.Find("input[name='name']").Length() == 0) {
		return Compat{}, fmt.Errorf("unrecognized compatibility search response")
	}
	result := parseCompatResults(body, title, searchURL)
	if result.Status == "error" {
		return Compat{}, fmt.Errorf("compatibility response has no verified Mac rating column")
	}
	result.CheckedAt = time.Now()
	result.MatchVersion = compatMatchVersion
	return result, nil
}

type compatCandidate struct {
	Name  string
	Href  string
	Stars int
}

func parseCompatResults(body []byte, title, searchURL string) Compat {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return noCompat(searchURL)
	}

	// The default search currently displays Mac ratings (Apple-marked header).
	// The platform=mac form filter returns zero results even for known games.
	// Validate the column explicitly instead of mistaking Linux ratings for Mac.
	if doc.Find("tr[id^='key_']").Length() > 0 && doc.Find("#teTable-app thead .fa-apple, #teTable-app thead .fa-brands.fa-apple").Length() == 0 {
		return Compat{Status: "error"}
	}
	var candidates []compatCandidate
	doc.Find("tr[id^='key_']").Each(func(_ int, sel *goquery.Selection) {
		link := sel.Find("td").Eq(0).Find("a").First()
		name := cleanText(link.Text())
		if name == "" {
			return
		}
		candidates = append(candidates, compatCandidate{
			Name:  name,
			Href:  link.AttrOr("href", ""),
			Stars: sel.Find("ul.star-rating-table").First().Find("li.active").Length(),
		})
	})

	best, _ := bestCompat(title, candidates)
	if best == nil {
		return noCompat(searchURL)
	}

	status, label := compatStatus(best.Stars)
	tooltip := "CrossOver Mac: no rating yet · matched “" + best.Name + "”"
	if best.Stars > 0 {
		tooltip = fmt.Sprintf("CrossOver Mac: %d/5 stars · matched “%s”", best.Stars, best.Name)
	}

	return Compat{
		Status:    status,
		Label:     label,
		Stars:     best.Stars,
		MatchName: best.Name,
		URL:       absURL(codeWeaversBase, best.Href),
		Tooltip:   tooltip,
		Checked:   true,
	}
}

func noCompat(searchURL string) Compat {
	return Compat{
		Status:  "none",
		Label:   "No report",
		URL:     searchURL,
		Tooltip: "No matching CrossOver report. Open the CodeWeavers search.",
		Checked: true,
	}
}

func bestCompat(title string, candidates []compatCandidate) (*compatCandidate, float64) {
	bestIndex := -1
	bestScore := 0.0
	for i := range candidates {
		// Editions that change the game or renderer need their own report.
		if !compatibleEditions(title, candidates[i].Name) || baseEditionTitle(title) != baseEditionTitle(candidates[i].Name) {
			continue
		}
		score := titleScore(title, candidates[i].Name)
		if baseEditionTitle(title) == baseEditionTitle(candidates[i].Name) && score < 0.97 {
			score = 0.97
		}
		if score > bestScore ||
			(score == bestScore && bestIndex >= 0 && len(candidates[i].Name) < len(candidates[bestIndex].Name)) {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex < 0 || bestScore < 0.9 {

		return nil, bestScore
	}
	return &candidates[bestIndex], bestScore
}

func titleScore(query, candidate string) float64 {
	q := normalizeTitle(query)
	c := normalizeTitle(candidate)
	if q == "" || c == "" {
		return 0
	}
	if q == c {
		return 1
	}

	qs := tokenSet(q)
	cs := tokenSet(c)
	if strings.Contains(" "+c+" ", " "+q+" ") {
		return 0.8 + 0.2*float64(len(qs))/float64(len(cs))
	}
	if isSubset(cs, qs) {
		return 0.55 * float64(len(cs)) / float64(len(qs))
	}

	shared := 0
	for token := range qs {
		if cs[token] {
			shared++
		}
	}
	union := len(qs) + len(cs) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer(
		"’", "'",
		"‘", "'",
		"´", "'",
		"`", "'",
		"“", `"`,
		"”", `"`,
		"&", " and ",
		"–", "-",
		"—", "-",
		"™", "",
		"®", "",
		"©", "",
	).Replace(s)
	s = strings.ReplaceAll(s, "'", "")

	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func tokenSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, token := range strings.Fields(s) {
		out[token] = true
	}
	return out
}

func isSubset(a, b map[string]bool) bool {
	if len(a) == 0 || len(a) > len(b) {
		return false
	}
	for token := range a {
		if !b[token] {
			return false
		}
	}
	return true
}

func compatStatus(stars int) (string, string) {
	switch stars {
	case 5:
		return "great", "Runs great"
	case 4:
		return "good", "Runs well"
	case 3:
		return "playable", "Some issues"
	case 2:
		return "issues", "Major issues"
	case 1:
		return "broken", "Broken"
	default:
		return "untested", "Untested"
	}
}

type compatEntry struct {
	compat Compat
	at     time.Time
}

type compatCache struct {
	mu      sync.Mutex
	entries map[string]compatEntry
}

func newCompatCache() *compatCache {
	return &compatCache{entries: make(map[string]compatEntry)}
}

func (c *compatCache) get(key string) (Compat, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.at) > 24*time.Hour {
		return Compat{}, false
	}
	return entry.compat, true
}

func (c *compatCache) put(key string, value Compat) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = compatEntry{compat: value, at: time.Now()}
}

// Only packaging suffixes are interchangeable. Preserve sequel numbers,
// remasters, remakes, Enhanced, DLC, and launcher names.
func baseEditionTitle(title string) string {
	title = normalizeTitle(title)
	suffixes := []string{"game of the year edition", "game of the year", "definitive edition", "deluxe edition", "ultimate edition", "complete edition", "standard edition", "legacy edition", "goty edition"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(title, " "+suffix) {
			return strings.TrimSuffix(title, " "+suffix)
		}
	}
	return title
}

func compatibleEditions(a, b string) bool {
	a, b = normalizeTitle(a), normalizeTitle(b)
	for _, marker := range []string{"enhanced", "remastered", "remake", "remaster", "definitive"} {
		has := func(s string) bool { return strings.Contains(" "+s+" ", " "+marker+" ") }
		if has(a) != has(b) {
			return false
		}
	}
	return true
}
