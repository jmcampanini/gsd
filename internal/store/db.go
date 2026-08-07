package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const busyTimeoutMS = 5000

type DB struct {
	database *sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		return nil, err
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	database, err := sql.Open("sqlite", dataSourceName(absolutePath))
	if err != nil {
		return nil, fmt.Errorf("configure database: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := migrate(ctx, database, migrations); err != nil {
		_ = database.Close()
		return nil, err
	}

	return &DB{database: database}, nil
}

func (d *DB) Close() error {
	return d.database.Close()
}

func dataSourceName(path string) string {
	location := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := location.Query()
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	location.RawQuery = query.Encode()

	return location.String()
}
