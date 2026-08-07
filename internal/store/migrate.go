package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
)

//go:embed all:migrations
var migrationFiles embed.FS

const gsdApplicationID = 0x47534431

type migration struct {
	revision int
	name     string
	sql      string
}

func loadMigrations(files fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return nil, fmt.Errorf("list database migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("database migration entry %q is not a regular file", entry.Name())
		}
		migrationPath := path.Join("migrations", entry.Name())
		revision, err := migrationRevision(migrationPath)
		if err != nil {
			return nil, err
		}
		contents, err := fs.ReadFile(files, migrationPath)
		if err != nil {
			return nil, fmt.Errorf("read database migration %s: %w", migrationPath, err)
		}
		migrations = append(migrations, migration{
			revision: revision,
			name:     migrationPath,
			sql:      string(contents),
		})
	}
	if err := validateMigrationChain(migrations); err != nil {
		return nil, err
	}

	return migrations, nil
}

func migrationRevision(migrationPath string) (int, error) {
	filename := path.Base(migrationPath)
	if len(filename) < len("0001_a.sql") || filename[4] != '_' ||
		!strings.HasSuffix(filename, ".sql") {
		return 0, fmt.Errorf("invalid database migration filename %q", filename)
	}
	revision, err := strconv.Atoi(filename[:4])
	if err != nil {
		return 0, fmt.Errorf("invalid database migration filename %q", filename)
	}

	return revision, nil
}

func validateMigrationChain(migrations []migration) error {
	if len(migrations) == 0 {
		return fmt.Errorf("database migration chain is empty")
	}
	for index, current := range migrations {
		expected := index + 1
		if current.revision != expected {
			return fmt.Errorf(
				"database migration %q has revision %d, want %d",
				current.name,
				current.revision,
				expected,
			)
		}
		if strings.TrimSpace(current.sql) == "" {
			return fmt.Errorf("database migration %q is empty", current.name)
		}
	}

	return nil
}

func migrate(ctx context.Context, database *sql.DB, migrations []migration) error {
	if err := validateMigrationChain(migrations); err != nil {
		return err
	}
	maxRevision := migrations[len(migrations)-1].revision
	// Lock-free fast path: applyMigration re-runs this ladder inside
	// BEGIN IMMEDIATE, where a concurrent open may have advanced the database.
	_, version, err := validateDatabaseState(ctx, database, maxRevision)
	if err != nil {
		return err
	}
	if version == maxRevision {
		return nil
	}

	for _, current := range migrations {
		if current.revision <= version {
			continue
		}
		if err := applyMigration(ctx, database, current, maxRevision); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(
	ctx context.Context,
	database *sql.DB,
	current migration,
	maxRevision int,
) error {
	return withinTransaction(
		ctx,
		database,
		fmt.Sprintf("database migration %04d", current.revision),
		"BEGIN IMMEDIATE",
		func(connection *sql.Conn) error {
			applicationID, version, err := validateDatabaseState(ctx, connection, maxRevision)
			if err != nil {
				return err
			}
			if version >= current.revision {
				return nil
			}
			if version != current.revision-1 {
				return fmt.Errorf(
					"database revision %d cannot apply migration revision %d",
					version,
					current.revision,
				)
			}
			if version == 0 {
				empty, err := databaseIsEmpty(ctx, connection)
				if err != nil {
					return err
				}
				if !empty {
					return apperr.New(
						apperr.Conflict,
						"database is not empty; delete your development database and try again",
						nil,
					)
				}
			}

			if _, err := connection.ExecContext(ctx, current.sql); err != nil {
				return fmt.Errorf("apply database migration %s: %w", current.name, err)
			}
			if applicationID != gsdApplicationID {
				if _, err := connection.ExecContext(
					ctx,
					fmt.Sprintf("PRAGMA application_id = %d", gsdApplicationID),
				); err != nil {
					return fmt.Errorf("stamp database application identity: %w", err)
				}
			}
			if _, err := connection.ExecContext(
				ctx,
				fmt.Sprintf("PRAGMA user_version = %d", current.revision),
			); err != nil {
				return fmt.Errorf("stamp database revision %d: %w", current.revision, err)
			}

			return nil
		},
	)
}

func databaseIsEmpty(ctx context.Context, connection *sql.Conn) (bool, error) {
	var objectCount int
	if err := connection.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM sqlite_schema
WHERE name NOT LIKE 'sqlite\_%' ESCAPE '\'
`).Scan(&objectCount); err != nil {
		return false, fmt.Errorf("inspect database schema: %w", err)
	}

	return objectCount == 0, nil
}

type rowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateDatabaseState(
	ctx context.Context,
	queryer rowQueryer,
	maxRevision int,
) (applicationID, version int, err error) {
	applicationID, err = databaseApplicationID(ctx, queryer)
	if err != nil {
		return 0, 0, err
	}
	version, err = userVersion(ctx, queryer)
	if err != nil {
		return 0, 0, err
	}
	if err := validateDatabaseIdentity(applicationID, version); err != nil {
		return 0, 0, err
	}
	if err := validateDatabaseRevision(version, maxRevision); err != nil {
		return 0, 0, err
	}

	return applicationID, version, nil
}

func databaseApplicationID(ctx context.Context, queryer rowQueryer) (int, error) {
	var applicationID int
	if err := queryer.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		return 0, fmt.Errorf("read database application identity: %w", err)
	}

	return applicationID, nil
}

func userVersion(ctx context.Context, queryer rowQueryer) (int, error) {
	var version int
	if err := queryer.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read database revision: %w", err)
	}

	return version, nil
}

func validateDatabaseIdentity(applicationID, version int) error {
	if applicationID == gsdApplicationID || applicationID == 0 && version == 0 {
		return nil
	}
	return apperr.New(
		apperr.Conflict,
		"database does not belong to gsd",
		nil,
	)
}

func validateDatabaseRevision(version, maxRevision int) error {
	if version < 0 {
		return apperr.New(
			apperr.Conflict,
			fmt.Sprintf("database revision %d is invalid", version),
			nil,
		)
	}
	if version > maxRevision {
		return apperr.New(
			apperr.Conflict,
			fmt.Sprintf(
				"gsd is older than this database (database revision %d, this gsd supports up to %d); upgrade gsd",
				version,
				maxRevision,
			),
			nil,
		)
	}

	return nil
}
