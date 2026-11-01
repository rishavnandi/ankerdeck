# AnkerDeck

An unofficial dashboard for AnkerGames' top and trending game charts, with each title matched to its CrossOver rating in the CodeWeavers compatibility database.

## Run

Requires Go 1.27 or newer.

```sh
go run .
```

Open <http://localhost:8080>. Configuration is optional:

| Variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address |
| `REFRESH_INTERVAL` | `15m` | How often the source charts refresh. Values below `1m` are raised to one minute. |
| `JINA_API_KEY` | unset | Optional key for the Jina Reader fallback. |
| `DB_PATH` | `ankerdeck.db` | SQLite database path. |

`GET /healthz` returns `ok`. The browser updates the home preview from `/boards` and refreshes full-list result fragments without a full page reload. Search and rating filters stay in place while results update. `POST /refresh` starts a manual refresh, limited to once every five minutes across restarts.

The home page shows the first five entries from each chart. `/top-games` shows up to 200 top games, following AnkerGames pagination. `/trending` shows the ranked entries supplied by AnkerGames. Its current page contains only 20 games, and requesting page 2 repeats those entries.

SQLite stores completed snapshots and restores the latest one on startup. Snapshots older than 30 days are removed during refresh. The theme follows the system preference until a visitor chooses light or dark mode.

## Data and matching

The server reads [AnkerGames Top Games](https://ankergames.net/top-games) and [Trending](https://ankergames.net/trending), then searches [CodeWeavers Compatibility](https://www.codeweavers.com/compatibility) for each distinct title. Compatibility results are cached for 24 hours from their original check time and restored from SQLite on restart. Matching preserves sequel numbers, remakes, remasters, and Enhanced editions; packaging suffixes such as Deluxe or Legacy Edition can resolve to the base title. Each badge links to its matched report and names the match in its tooltip. Matching rules are versioned so old results are rechecked when the matcher changes.

AnkerGames may return a Cloudflare challenge to direct requests. The fetcher then uses the Jina Reader service to retrieve the HTML. A Jina API key is optional, but its service limits can affect refreshes. If either chart source fails, AnkerDeck keeps the last good board and retries on the next refresh. The default CodeWeavers search currently supplies Mac ratings, identified by an Apple-marked table header. The parser verifies that marker and rejects other platforms or unexpected pages. Its `platform=mac` search-form filter returned empty results for known games during verification, so it is deliberately not used. Compatibility checks prioritize both charts by rank, run in two bounded workers, and have a separate deadline from chart retrieval. Charts are saved before enrichment, and a failed pagination request preserves the previous complete chart.

The included `testdata/` HTML files are captured source pages used by the parser tests.
