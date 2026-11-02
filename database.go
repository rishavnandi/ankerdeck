package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type snapshotDB struct {
	db       *sql.DB
	postgres bool
}

func openSnapshotDB(path string) (*snapshotDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS runs (
		id INTEGER PRIMARY KEY,
		captured_at TEXT NOT NULL,
		payload BLOB NOT NULL
	);`); err == nil {
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS runs_captured_at ON runs(captured_at)`)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return &snapshotDB{db: db}, nil
}

func (d *snapshotDB) latest(ctx context.Context) (Snapshot, error) {
	var payload []byte
	err := d.db.QueryRowContext(ctx, `SELECT payload FROM runs ORDER BY id DESC LIMIT 1`).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	err = json.Unmarshal(payload, &snapshot)
	return snapshot, err
}

func (d *snapshotDB) save(ctx context.Context, snapshot Snapshot) error {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if _, err = d.db.ExecContext(ctx, d.query(`INSERT INTO runs(captured_at, payload) VALUES(?, ?)`), snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano), payload); err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx, d.query(`DELETE FROM runs WHERE captured_at < ?`), time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339Nano))
	return err
}

func (d *snapshotDB) query(query string) string {
	if !d.postgres {
		return query
	}
	n := 0
	return replacePlaceholders(query, &n)
}

func replacePlaceholders(query string, n *int) string {
	var out strings.Builder
	for _, r := range query {
		if r == '?' {
			*n++
			fmt.Fprintf(&out, "$%d", *n)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func openPostgresSnapshotDB(ctx context.Context, connection string) (*snapshotDB, error) {
	db, err := sql.Open("pgx", connection)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS runs (id BIGSERIAL PRIMARY KEY, captured_at TEXT NOT NULL, payload BYTEA NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS runs_captured_at ON runs(captured_at)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &snapshotDB{db: db, postgres: true}, nil
}

func (d *snapshotDB) close() error { return d.db.Close() }
