# AnkerDeck

An unofficial dashboard for AnkerGames' top and trending game charts, with each title matched to its CrossOver rating in the CodeWeavers compatibility database. The site renders saved chart snapshots; it does not fetch or update charts automatically.

## Run locally

Requires Go 1.27 or newer.

```sh
go run .
```

Open <http://localhost:8080>. Configuration is optional:

| Variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address |
| `DB_PATH` | `ankerdeck.db` | SQLite database path |
| `DATABASE_URL` | unset | PostgreSQL connection URL; when set, it replaces SQLite |

`GET /healthz` returns `ok`. The home page shows the first five entries from each chart. `/top-games` shows up to 200 top games, and `/trending` shows the saved trending list. Search and CrossOver rating filters run in the browser.

## Import a weekly snapshot

Fetch the Top Games and Trending lists manually, then save both lists and their capture timestamps in one snapshot JSON file. The JSON fields match `Snapshot` and `Game` in `snapshot.go`; each game's `Compat` object contains the static badge fields `Status`, `Label`, `Stars`, `MatchName`, `URL`, and `Tooltip`.

For local SQLite, import a snapshot with:

```sh
go run . -seed path/to/snapshot.json
```

To update the production Neon database, set `DATABASE_URL` in your shell and pass the snapshot file to the helper:

```sh
export DATABASE_URL='<Neon pooled connection URL>'
./scripts/seed-weekly.sh data/weekly-snapshot.json
```

The helper requires `DATABASE_URL`, imports the supplied file, and exits. It never prints the connection URL. Keep credentials in your shell or a secret manager; do not put them in the snapshot or Git.

## Data and compatibility ratings

The snapshot stores both charts, their update times, game metadata, and CrossOver badge details. The application displays those values as imported and does not query AnkerGames or CodeWeavers. The compatibility legend describes the badge labels. Ratings are for CrossOver on Mac; select a badge to open its report.

The `testdata/` HTML files are captured source pages retained as reference fixtures.

## Vercel

The app uses Vercel's Go framework preset and listens on `PORT`. Set `DATABASE_URL` to the production Neon pooled PostgreSQL connection URL. Vercel reloads the saved snapshot from the database for each page request. There is no cron job or refresh endpoint.

Deploy with `vercel --prod` after configuring the production environment. The GitHub repository is linked to the Vercel project for subsequent deployments from `main`.
