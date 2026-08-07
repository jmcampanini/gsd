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
	for _, digit := range filename[:4] {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("invalid database migration filename %q", filename)
		}
	}
	revision, err := strconv.Atoi(filename[:4])
	if err != nil {
		return 0, fmt.Errorf("parse database migration revision %q: %w", filename[:4], err)
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
		if err := validateMigrationSQL(current); err != nil {
			return err
		}
	}

	return nil
}

func validateMigrationSQL(current migration) error {
	keywords, err := migrationStatementKeywords(current.sql)
	if err != nil {
		return fmt.Errorf("parse database migration %q: %w", current.name, err)
	}
	for _, keyword := range keywords {
		switch keyword {
		case "ALTER", "CREATE", "DROP":
		default:
			return fmt.Errorf(
				"database migration %q contains non-DDL statement %s",
				current.name,
				keyword,
			)
		}
	}

	return nil
}

func migrationStatementKeywords(sqlText string) ([]string, error) {
	keywords := make([]string, 0)
	statementStart := true
	for index := 0; index < len(sqlText); {
		switch {
		case isMigrationSpace(sqlText[index]):
			index++
		case strings.HasPrefix(sqlText[index:], "--"):
			index = skipMigrationLineComment(sqlText, index+2)
		case strings.HasPrefix(sqlText[index:], "/*"):
			var err error
			index, err = skipMigrationBlockComment(sqlText, index+2)
			if err != nil {
				return nil, err
			}
		case sqlText[index] == ';':
			statementStart = true
			index++
		case statementStart:
			start := index
			for index < len(sqlText) && isMigrationKeywordCharacter(sqlText[index]) {
				index++
			}
			if start == index {
				return nil, fmt.Errorf("statement starts with %q", sqlText[index])
			}
			keyword := strings.ToUpper(sqlText[start:index])
			keywords = append(keywords, keyword)
			statementStart = false
			if keyword == "CREATE" {
				trigger, err := migrationCreatesTrigger(sqlText, index)
				if err != nil {
					return nil, err
				}
				if trigger {
					index, err = skipMigrationTrigger(sqlText, index)
					if err != nil {
						return nil, err
					}
					statementStart = true
				}
			}
		case sqlText[index] == '\'' || sqlText[index] == '"' || sqlText[index] == '`':
			var err error
			index, err = skipMigrationQuoted(sqlText, index, sqlText[index])
			if err != nil {
				return nil, err
			}
		case sqlText[index] == '[':
			var err error
			index, err = skipMigrationBracketed(sqlText, index)
			if err != nil {
				return nil, err
			}
		default:
			index++
		}
	}
	if len(keywords) == 0 {
		return nil, fmt.Errorf("contains no statements")
	}

	return keywords, nil
}

func isMigrationSpace(current byte) bool {
	switch current {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

func isMigrationKeywordCharacter(current byte) bool {
	return current >= 'A' && current <= 'Z' || current >= 'a' && current <= 'z'
}

func skipMigrationLineComment(sqlText string, index int) int {
	for index < len(sqlText) && sqlText[index] != '\n' {
		index++
	}
	return index
}

func skipMigrationBlockComment(sqlText string, index int) (int, error) {
	end := strings.Index(sqlText[index:], "*/")
	if end < 0 {
		return 0, fmt.Errorf("unterminated block comment")
	}
	return index + end + 2, nil
}

func skipMigrationQuoted(sqlText string, index int, quote byte) (int, error) {
	for index++; index < len(sqlText); index++ {
		if sqlText[index] != quote {
			continue
		}
		if index+1 < len(sqlText) && sqlText[index+1] == quote {
			index++
			continue
		}
		return index + 1, nil
	}
	return 0, fmt.Errorf("unterminated quoted value")
}

func skipMigrationBracketed(sqlText string, index int) (int, error) {
	end := strings.IndexByte(sqlText[index+1:], ']')
	if end < 0 {
		return 0, fmt.Errorf("unterminated bracketed identifier")
	}
	return index + end + 2, nil
}

func migrationCreatesTrigger(sqlText string, index int) (bool, error) {
	word, index, err := nextMigrationWord(sqlText, index)
	if err != nil {
		return false, err
	}
	if word == "TEMP" || word == "TEMPORARY" {
		word, _, err = nextMigrationWord(sqlText, index)
		if err != nil {
			return false, err
		}
	}
	return word == "TRIGGER", nil
}

func nextMigrationWord(sqlText string, index int) (string, int, error) {
	index, err := skipMigrationTrivia(sqlText, index)
	if err != nil {
		return "", 0, err
	}
	if index == len(sqlText) {
		return "", 0, fmt.Errorf("incomplete CREATE statement")
	}
	start := index
	for index < len(sqlText) && isMigrationKeywordCharacter(sqlText[index]) {
		index++
	}
	if start == index {
		return "", 0, fmt.Errorf("want keyword after CREATE")
	}
	return strings.ToUpper(sqlText[start:index]), index, nil
}

func skipMigrationTrivia(sqlText string, index int) (int, error) {
	for index < len(sqlText) {
		switch {
		case isMigrationSpace(sqlText[index]):
			index++
		case strings.HasPrefix(sqlText[index:], "--"):
			index = skipMigrationLineComment(sqlText, index+2)
		case strings.HasPrefix(sqlText[index:], "/*"):
			var err error
			index, err = skipMigrationBlockComment(sqlText, index+2)
			if err != nil {
				return 0, err
			}
		default:
			return index, nil
		}
	}
	return index, nil
}

func skipMigrationTrigger(sqlText string, index int) (int, error) {
	bodyStarted := false
	caseDepth := 0
	for index < len(sqlText) {
		switch {
		case strings.HasPrefix(sqlText[index:], "--"):
			index = skipMigrationLineComment(sqlText, index+2)
		case strings.HasPrefix(sqlText[index:], "/*"):
			var err error
			index, err = skipMigrationBlockComment(sqlText, index+2)
			if err != nil {
				return 0, err
			}
		case sqlText[index] == '\'' || sqlText[index] == '"' || sqlText[index] == '`':
			var err error
			index, err = skipMigrationQuoted(sqlText, index, sqlText[index])
			if err != nil {
				return 0, err
			}
		case sqlText[index] == '[':
			var err error
			index, err = skipMigrationBracketed(sqlText, index)
			if err != nil {
				return 0, err
			}
		case isMigrationKeywordCharacter(sqlText[index]):
			start := index
			for index < len(sqlText) && isMigrationKeywordCharacter(sqlText[index]) {
				index++
			}
			word := strings.ToUpper(sqlText[start:index])
			switch {
			case !bodyStarted && word == "BEGIN":
				bodyStarted = true
			case bodyStarted && word == "CASE":
				caseDepth++
			case bodyStarted && word == "END" && caseDepth > 0:
				caseDepth--
			case bodyStarted && word == "END":
				var err error
				index, err = skipMigrationTrivia(sqlText, index)
				if err != nil {
					return 0, err
				}
				if index == len(sqlText) {
					return index, nil
				}
				if sqlText[index] != ';' {
					return 0, fmt.Errorf("trigger END is not followed by a statement terminator")
				}
				return index + 1, nil
			}
		default:
			index++
		}
	}
	return 0, fmt.Errorf("incomplete CREATE TRIGGER statement")
}

func migrate(ctx context.Context, database *sql.DB, migrations []migration) error {
	if err := validateMigrationChain(migrations); err != nil {
		return err
	}
	maxRevision := migrations[len(migrations)-1].revision
	version, err := userVersion(ctx, database)
	if err != nil {
		return err
	}
	if err := validateDatabaseRevision(version, maxRevision); err != nil {
		return err
	}
	if version == maxRevision {
		return nil
	}

	storage := &DB{database: database}
	for _, current := range migrations {
		if current.revision <= version {
			continue
		}
		if err := applyMigration(ctx, storage, current, maxRevision); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(
	ctx context.Context,
	storage *DB,
	current migration,
	maxRevision int,
) error {
	return withinImmediateTransaction(
		ctx,
		storage,
		fmt.Sprintf("database migration %04d", current.revision),
		func(connection *sql.Conn) error {
			version, err := userVersion(ctx, connection)
			if err != nil {
				return err
			}
			if err := validateDatabaseRevision(version, maxRevision); err != nil {
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

			temporaryObjects, err := temporarySchemaObjectCount(ctx, connection)
			if err != nil {
				return err
			}
			if _, err := connection.ExecContext(ctx, current.sql); err != nil {
				return fmt.Errorf("apply database migration %s: %w", current.name, err)
			}
			currentTemporaryObjects, err := temporarySchemaObjectCount(ctx, connection)
			if err != nil {
				return err
			}
			if currentTemporaryObjects != temporaryObjects {
				return fmt.Errorf("database migration %s changed the temporary schema", current.name)
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

func temporarySchemaObjectCount(ctx context.Context, connection *sql.Conn) (int, error) {
	var objectCount int
	if err := connection.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM sqlite_temp_schema
WHERE name NOT LIKE 'sqlite\_%' ESCAPE '\'
`).Scan(&objectCount); err != nil {
		return 0, fmt.Errorf("inspect temporary database schema: %w", err)
	}
	return objectCount, nil
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

func userVersion(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (int, error) {
	var version int
	if err := queryer.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read database revision: %w", err)
	}

	return version, nil
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
