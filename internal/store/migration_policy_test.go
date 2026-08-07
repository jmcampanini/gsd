package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestEmbeddedMigrationChainSatisfiesMigrationPolicy(t *testing.T) {
	t.Parallel()

	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		t.Fatalf("loadMigrations() error = %v", err)
	}
	if err := lintMigrationPolicy(context.Background(), openRawMigrationDatabase(t), migrations); err != nil {
		t.Fatalf("embedded migration policy: %v", err)
	}
}

func TestMigrationPolicyAllowsTopLevelDDL(t *testing.T) {
	t.Parallel()

	migrations := []migration{
		{
			revision: 1,
			name:     "0001_first.sql",
			sql:      "-- baseline\nCREATE TABLE first (value TEXT DEFAULT ';');",
		},
		{
			revision: 2,
			name:     "0002_second.sql",
			sql:      "/* next */ CREATE TRIGGER first_update AFTER UPDATE ON first BEGIN UPDATE first SET value = CASE WHEN NEW.value IS NULL THEN ';' ELSE NEW.value END; END; CREATE TABLE second (id INTEGER);",
		},
		{
			revision: 3,
			name:     "0003_third.sql",
			sql:      "ALTER TABLE second ADD COLUMN note TEXT; DROP TRIGGER first_update;",
		},
	}

	if err := lintMigrationPolicy(context.Background(), openRawMigrationDatabase(t), migrations); err != nil {
		t.Fatalf("lintMigrationPolicy() error = %v", err)
	}
}

func TestMigrationPolicyRejectsNonDDLStatements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sql       string
		wantError string
	}{
		{
			name:      "data manipulation",
			sql:       "DELETE FROM tasks;",
			wantError: "non-DDL statement DELETE",
		},
		{
			name:      "transaction control",
			sql:       "CREATE TABLE first (id INTEGER); COMMIT;",
			wantError: "non-DDL statement COMMIT",
		},
		{
			name:      "runner owned pragma",
			sql:       "PRAGMA user_version = 1;",
			wantError: "non-DDL statement PRAGMA",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			migrations := []migration{{revision: 1, name: "0001_first.sql", sql: test.sql}}
			err := lintMigrationPolicy(context.Background(), openRawMigrationDatabase(t), migrations)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Errorf("lintMigrationPolicy() error = %v, want containing %q", err, test.wantError)
			}
		})
	}
}

func TestMigrationPolicyRejectsTemporarySchemaChanges(t *testing.T) {
	t.Parallel()

	migrations := []migration{
		{revision: 1, name: "0001_temporary.sql", sql: "CREATE TEMP TABLE transient (id INTEGER);"},
	}

	err := lintMigrationPolicy(context.Background(), openRawMigrationDatabase(t), migrations)
	if err == nil || !strings.Contains(err.Error(), "changed the temporary schema") {
		t.Fatalf("lintMigrationPolicy() error = %v, want temporary-schema rejection", err)
	}
}

// lintMigrationPolicy enforces the authoring rules the runner does not check
// at runtime: migrations contain top-level DDL only and leave the temporary
// schema unchanged.
func lintMigrationPolicy(ctx context.Context, database *sql.DB, migrations []migration) error {
	if err := validateMigrationChain(migrations); err != nil {
		return err
	}
	for _, current := range migrations {
		if err := validateMigrationSQL(current); err != nil {
			return err
		}
	}
	for index, current := range migrations {
		temporaryObjects, err := temporarySchemaObjectCount(ctx, database)
		if err != nil {
			return err
		}
		if err := migrate(ctx, database, migrations[:index+1]); err != nil {
			return fmt.Errorf("apply migrations through revision %d: %w", current.revision, err)
		}
		currentTemporaryObjects, err := temporarySchemaObjectCount(ctx, database)
		if err != nil {
			return err
		}
		if currentTemporaryObjects != temporaryObjects {
			return fmt.Errorf("database migration %s changed the temporary schema", current.name)
		}
	}

	return nil
}

func temporarySchemaObjectCount(ctx context.Context, queryer rowQueryer) (int, error) {
	var objectCount int
	if err := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM sqlite_temp_schema
WHERE name NOT LIKE 'sqlite\_%' ESCAPE '\'
`).Scan(&objectCount); err != nil {
		return 0, fmt.Errorf("inspect temporary database schema: %w", err)
	}
	return objectCount, nil
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
