package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type snapshotDB struct{ db *sql.DB }

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
	if err == nil {
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS limits (name TEXT PRIMARY KEY, at TEXT NOT NULL)`)
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
	if _, err = d.db.ExecContext(ctx, `INSERT INTO runs(captured_at, payload) VALUES(?, ?)`, snapshot.UpdatedAt.UTC().Format(time.RFC3339Nano), payload); err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx, `DELETE FROM runs WHERE captured_at < ?`, time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339Nano))
	return err
}

func (d *snapshotDB) takeManualSlot(ctx context.Context, now time.Time, window time.Duration) (time.Duration, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var lastText string
	err = tx.QueryRowContext(ctx, `SELECT at FROM limits WHERE name='manual-refresh'`).Scan(&lastText)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if err == nil {
		last, parseErr := time.Parse(time.RFC3339Nano, lastText)
		if parseErr != nil {
			return 0, parseErr
		}
		if now.Sub(last) < window {
			return window - now.Sub(last), nil
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO limits(name, at) VALUES('manual-refresh', ?) ON CONFLICT(name) DO UPDATE SET at=excluded.at`, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return 0, err
	}
	return 0, tx.Commit()
}

func (d *snapshotDB) close() error { return d.db.Close() }
