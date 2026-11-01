package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func htmlResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestTopRejectsPartialAndRepeatedPages(t *testing.T) {
	fixture, err := os.ReadFile("testdata/top.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"failure", "repeat"} {
		t.Run(mode, func(t *testing.T) {
			fetcher := NewFetcher("")
			fetcher.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == topGamesURL || mode == "repeat" {
					return htmlResponse(string(fixture)), nil
				}
				return nil, fmt.Errorf("source unavailable")
			})
			games, err := fetcher.Top(context.Background())
			if err == nil || len(games) > 0 {
				t.Fatalf("partial chart published: %d games, err=%v", len(games), err)
			}
		})
	}
}

func TestCompatibilityLookupVerifiesMacAndRejectsChangedPages(t *testing.T) {
	fixture, err := os.ReadFile("testdata/compat-search.html")
	if err != nil {
		t.Fatal(err)
	}
	client := &CompatClient{fetch: func(ctx context.Context, raw string) ([]byte, error) {
		request, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
		if err != nil {
			return nil, err
		}
		if request.URL.Query().Get("name") != "Minecraft" {
			t.Error("lookup omitted game title")
		}
		return fixture, nil
	}}
	result, err := client.Lookup(context.Background(), "Minecraft")
	if err != nil || !result.Checked || result.CheckedAt.IsZero() || result.MatchVersion != compatMatchVersion {
		t.Fatalf("lookup = %+v, %v", result, err)
	}
	linux := strings.ReplaceAll(string(fixture), "fa-apple", "fa-linux")
	client.fetch = func(context.Context, string) ([]byte, error) { return []byte(linux), nil }
	if _, err := client.Lookup(context.Background(), "Minecraft"); err == nil {
		t.Fatal("accepted Linux ratings as Mac ratings")
	}
	client.fetch = func(context.Context, string) ([]byte, error) {
		return []byte(`<html><h1>Service unavailable</h1></html>`), nil
	}
	if _, err := client.Lookup(context.Background(), "Minecraft"); err == nil {
		t.Fatal("unexpected HTML was treated as a missing game")
	}
}

func TestReaderChallengeIsNotAccepted(t *testing.T) {
	fetcher := NewFetcher("")
	fetcher.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return htmlResponse(`<html>Just a moment...</html>`), nil })
	if _, err := fetcher.Get(context.Background(), topGamesURL); err == nil {
		t.Fatal("accepted a challenge as chart data")
	}
}
