package store

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsValidatesEmbeddedCatalog(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		files     fstest.MapFS
		wantCount int
		wantError string
	}{
		{
			name: "valid",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql": &fstest.MapFile{Data: []byte("-- baseline\nCREATE TABLE first (value TEXT DEFAULT ';');")},
				"migrations/0002_second.sql":   &fstest.MapFile{Data: []byte("/* next */ CREATE TRIGGER first_update AFTER UPDATE ON first BEGIN UPDATE first SET value = CASE WHEN NEW.value IS NULL THEN ';' ELSE NEW.value END; END; CREATE TABLE second (id INTEGER);")},
			},
			wantCount: 2,
		},
		{
			name: "malformed filename",
			files: fstest.MapFS{
				"migrations/1_baseline.sql": &fstest.MapFile{Data: []byte("CREATE TABLE first (id INTEGER);")},
			},
			wantError: "invalid database migration filename",
		},
		{
			name: "malformed extension",
			files: fstest.MapFS{
				"migrations/0001_baseline.sq": &fstest.MapFile{Data: []byte("CREATE TABLE first (id INTEGER);")},
			},
			wantError: "invalid database migration filename",
		},
		{
			name: "gap",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql": &fstest.MapFile{Data: []byte("CREATE TABLE first (id INTEGER);")},
				"migrations/0003_third.sql":    &fstest.MapFile{Data: []byte("CREATE TABLE third (id INTEGER);")},
			},
			wantError: "want 2",
		},
		{
			name: "duplicate revision",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql":  &fstest.MapFile{Data: []byte("CREATE TABLE first (id INTEGER);")},
				"migrations/0001_duplicate.sql": &fstest.MapFile{Data: []byte("CREATE TABLE duplicate (id INTEGER);")},
			},
			wantError: "want 2",
		},
		{
			name: "empty file",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql": &fstest.MapFile{},
			},
			wantError: "is empty",
		},
		{
			name: "data manipulation",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql": &fstest.MapFile{Data: []byte("DELETE FROM tasks;")},
			},
			wantError: "non-DDL statement DELETE",
		},
		{
			name: "transaction control",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql": &fstest.MapFile{Data: []byte("CREATE TABLE first (id INTEGER); COMMIT;")},
			},
			wantError: "non-DDL statement COMMIT",
		},
		{
			name: "runner owned pragma",
			files: fstest.MapFS{
				"migrations/0001_baseline.sql": &fstest.MapFile{Data: []byte("PRAGMA user_version = 1;")},
			},
			wantError: "non-DDL statement PRAGMA",
		},
		{
			name: "empty chain",
			files: fstest.MapFS{
				"migrations": &fstest.MapFile{Mode: fs.ModeDir},
			},
			wantError: "chain is empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			migrations, err := loadMigrations(test.files)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("loadMigrations() error = %v", err)
				}
				if len(migrations) != test.wantCount {
					t.Errorf("loadMigrations() count = %d, want %d", len(migrations), test.wantCount)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("loadMigrations() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestMigrateAppliesFullChainToEmptyDatabase(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openRawMigrationDatabase(t)
	migrations := []migration{
		{revision: 1, name: "0001_first.sql", sql: "CREATE TABLE first (id INTEGER);"},
		{revision: 2, name: "0002_second.sql", sql: "CREATE VIEW second AS SELECT id FROM first;"},
	}

	if err := migrate(ctx, database, migrations); err != nil {
		t.Fatalf("migrate() error = %v", err)
	}
	if version := migrationTestUserVersion(t, database); version != 2 {
		t.Errorf("user_version = %d, want 2", version)
	}
	for _, object := range []string{"first", "second"} {
		if count := migrationTestCount(
			t,
			database,
			"SELECT COUNT(*) FROM sqlite_schema WHERE name = ?",
			object,
		); count != 1 {
			t.Errorf("schema object %s count = %d, want 1", object, count)
		}
	}
}

func TestMigrateRejectsTemporarySchemaObjects(t *testing.T) {
	t.Parallel()

	database := openRawMigrationDatabase(t)
	migrations := []migration{
		{revision: 1, name: "0001_temporary.sql", sql: "CREATE TEMP TABLE transient (id INTEGER);"},
	}

	err := migrate(context.Background(), database, migrations)
	if err == nil || !strings.Contains(err.Error(), "changed the temporary schema") {
		t.Fatalf("migrate() error = %v, want temporary-schema rejection", err)
	}
	if version := migrationTestUserVersion(t, database); version != 0 {
		t.Errorf("user_version after rejection = %d, want 0", version)
	}
	if count := migrationTestCount(
		t,
		database,
		"SELECT COUNT(*) FROM sqlite_temp_schema WHERE name = 'transient'",
	); count != 0 {
		t.Errorf("temporary table count after rejection = %d, want 0", count)
	}
}

func TestMigrateAppliesOnlyPendingMigrations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openRawMigrationDatabase(t)
	if _, err := database.ExecContext(ctx, `
CREATE TABLE first (id INTEGER);
INSERT INTO first (id) VALUES (7);
PRAGMA user_version = 1;
`); err != nil {
		t.Fatalf("prepare revision 1 database: %v", err)
	}
	migrations := []migration{
		{revision: 1, name: "0001_first.sql", sql: "CREATE TABLE first (id INTEGER);"},
		{revision: 2, name: "0002_second.sql", sql: "CREATE TABLE second (id INTEGER);"},
	}

	if err := migrate(ctx, database, migrations); err != nil {
		t.Fatalf("migrate() error = %v", err)
	}
	if version := migrationTestUserVersion(t, database); version != 2 {
		t.Errorf("user_version = %d, want 2", version)
	}
	if count := migrationTestCount(
		t,
		database,
		"SELECT COUNT(*) FROM sqlite_schema WHERE name = 'second'",
	); count != 1 {
		t.Errorf("second table count = %d, want 1", count)
	}
	if count := migrationTestCount(t, database, "SELECT COUNT(*) FROM first WHERE id = 7"); count != 1 {
		t.Errorf("preserved first-table row count = %d, want 1", count)
	}
}

func TestMigrateRollsBackFailedMigrationAndResumes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	database := openRawMigrationDatabase(t)
	failing := []migration{
		{revision: 1, name: "0001_first.sql", sql: "CREATE TABLE first (id INTEGER);"},
		{
			revision: 2,
			name:     "0002_failing.sql",
			sql:      "CREATE TABLE partial (id INTEGER); CREATE TABLE first (id INTEGER);",
		},
	}

	if err := migrate(ctx, database, failing); err == nil {
		t.Fatal("migrate(failing) error = nil, want migration failure")
	}
	if version := migrationTestUserVersion(t, database); version != 1 {
		t.Errorf("user_version after failure = %d, want 1", version)
	}
	if count := migrationTestCount(
		t,
		database,
		"SELECT COUNT(*) FROM sqlite_schema WHERE name = 'first'",
	); count != 1 {
		t.Errorf("first table count after failure = %d, want 1", count)
	}
	if count := migrationTestCount(
		t,
		database,
		"SELECT COUNT(*) FROM sqlite_schema WHERE name = 'partial'",
	); count != 0 {
		t.Errorf("partial table count after failure = %d, want 0", count)
	}

	fixed := []migration{
		{revision: 1, name: "0001_first.sql", sql: "CREATE TABLE first (id INTEGER);"},
		{revision: 2, name: "0002_fixed.sql", sql: "CREATE TABLE partial (id INTEGER);"},
	}
	if err := migrate(ctx, database, fixed); err != nil {
		t.Fatalf("migrate(fixed) error = %v", err)
	}
	if version := migrationTestUserVersion(t, database); version != 2 {
		t.Errorf("user_version after resume = %d, want 2", version)
	}
	if count := migrationTestCount(
		t,
		database,
		"SELECT COUNT(*) FROM sqlite_schema WHERE name = 'partial'",
	); count != 1 {
		t.Errorf("partial table count after resume = %d, want 1", count)
	}
}

func openRawMigrationDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", dataSourceName(filepath.Join(t.TempDir(), "gsd.db")))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if err := database.PingContext(context.Background()); err != nil {
		t.Fatalf("PingContext() error = %v", err)
	}

	return database
}

func migrationTestUserVersion(t *testing.T, database *sql.DB) int {
	t.Helper()

	version, err := userVersion(context.Background(), database)
	if err != nil {
		t.Fatalf("userVersion() error = %v", err)
	}
	return version
}

func migrationTestCount(t *testing.T, database *sql.DB, query string, arguments ...any) int {
	t.Helper()

	var count int
	if err := database.QueryRow(query, arguments...).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	return count
}
