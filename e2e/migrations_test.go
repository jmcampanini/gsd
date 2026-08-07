package e2e

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	_ "modernc.org/sqlite"
)

func TestMigrationWorkflow(t *testing.T) {
	t.Parallel()

	directory, err := os.MkdirTemp(workDir, "migrations-")
	if err != nil {
		t.Fatalf("create migration workflow directory: %v", err)
	}
	databasePath := filepath.Join(directory, "gsd.db")
	created := decodeTask(t, runGSD(t, "add", "persistent", "--db", databasePath, "--json"))
	inbox := decodeTasks(t, runGSD(t, "inbox", "--db", databasePath, "--json"))
	if len(inbox) != 1 || inbox[0].ID != created.ID || inbox[0].Title != created.Title {
		t.Errorf("inbox after separate invocation = %#v, want persisted task %#v", inbox, created)
	}

	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open migrated database: %v", err)
	}
	var version int
	if err := database.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		_ = database.Close()
		t.Fatalf("read migrated database revision: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}
	if version != 1 {
		t.Errorf("migrated database revision = %d, want 1", version)
	}

	futurePath := filepath.Join(directory, "future.db")
	futureDatabase, err := sql.Open("sqlite", futurePath)
	if err != nil {
		t.Fatalf("open future database: %v", err)
	}
	if _, err := futureDatabase.Exec("PRAGMA user_version = 2"); err != nil {
		_ = futureDatabase.Close()
		t.Fatalf("stamp future database revision: %v", err)
	}
	if err := futureDatabase.Close(); err != nil {
		t.Fatalf("close future database: %v", err)
	}

	const futureMessage = "gsd is older than this database (database revision 2, this gsd supports up to 1); upgrade gsd"
	assertMigrationJSONError(
		t,
		runGSD(t, "inbox", "--db", futurePath, "--json"),
		apperr.Conflict,
		futureMessage,
	)

	futureDatabase, err = sql.Open("sqlite", futurePath)
	if err != nil {
		t.Fatalf("reopen future database: %v", err)
	}
	defer func() { _ = futureDatabase.Close() }()
	var objectCount int
	if err := futureDatabase.QueryRow(`
SELECT COUNT(*) FROM sqlite_schema WHERE name NOT LIKE 'sqlite\_%' ESCAPE '\'
`).Scan(&objectCount); err != nil {
		t.Fatalf("inspect refused future database: %v", err)
	}
	if err := futureDatabase.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("recheck future database revision: %v", err)
	}
	if version != 2 || objectCount != 0 {
		t.Errorf("future database after refusal = revision %d/object count %d, want 2/0", version, objectCount)
	}
}

func assertMigrationJSONError(
	t *testing.T,
	result processResult,
	wantCode apperr.Code,
	wantMessage string,
) {
	t.Helper()
	assertJSONError(t, result, wantCode)

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode migration error: %v", err)
	}
	if envelope.Error.Message != wantMessage {
		t.Errorf("migration error message = %q, want %q", envelope.Error.Message, wantMessage)
	}
}
