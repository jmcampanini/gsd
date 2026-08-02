package e2e

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type tagRow struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type listedTagRow struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	UsageCount int64  `json:"usage_count"`
}

type tagDeletion struct {
	Tag      tagRow `json:"tag"`
	Detached int64  `json:"detached"`
}

func TestTagAdministrationAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "tags", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	bare := runGSD(t, "tags", "--db", databasePath)
	if bare.exitCode != 2 || bare.stdout != "" || bare.stderr == "" {
		t.Errorf("bare tags = %#v, want stderr-only usage error", bare)
	}
	if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("bare tags database stat error = %v, want not exist", err)
	}

	errands := decodeTagRow(t, runJSON("tags", "add", "errands"))
	home := decodeTagRow(t, runJSON("tags", "add", "home"))
	if errands.ID != 1 || errands.Title != "errands" || errands.CreatedAt == "" || errands.UpdatedAt == "" {
		t.Errorf("errands tag = %#v, want complete first tag row", errands)
	}
	if home.ID != 2 || home.Title != "home" || home.CreatedAt == "" || home.UpdatedAt == "" {
		t.Errorf("home tag = %#v, want complete second tag row", home)
	}

	listed := decodeListedTagRows(t, runJSON("tags", "list"))
	if len(listed) != 2 || listed[0].ID != errands.ID || listed[0].Title != errands.Title ||
		listed[0].UsageCount != 0 || listed[1].ID != home.ID || listed[1].Title != home.Title ||
		listed[1].UsageCount != 0 {
		t.Errorf("listed tags = %#v, want errands then home with zero usage", listed)
	}

	conflictMessage := decodeTagError(
		t,
		runJSON("tags", "add", "Errands"),
		apperr.Conflict,
	)
	if !strings.Contains(conflictMessage, errands.Title) {
		t.Errorf("conflict message = %q, want stored spelling %q", conflictMessage, errands.Title)
	}

	renamed := decodeTagRow(t, runJSON("tags", "rename", "errands", "out-and-about"))
	if renamed.ID != errands.ID || renamed.Title != "out-and-about" || renamed.CreatedAt != errands.CreatedAt {
		t.Errorf("renamed errands = %#v, want in-place rename of %#v", renamed, errands)
	}
	caseRenamed := decodeTagRow(t, runJSON("tags", "rename", "home", "HOME"))
	if caseRenamed.ID != home.ID || caseRenamed.Title != "HOME" || caseRenamed.CreatedAt != home.CreatedAt {
		t.Errorf("case-renamed home = %#v, want in-place rename of %#v", caseRenamed, home)
	}

	listed = decodeListedTagRows(t, runJSON("tags", "list"))
	if len(listed) != 2 || listed[0].ID != caseRenamed.ID || listed[0].Title != caseRenamed.Title ||
		listed[0].UsageCount != 0 || listed[1].ID != renamed.ID || listed[1].Title != renamed.Title ||
		listed[1].UsageCount != 0 {
		t.Errorf("listed renamed tags = %#v, want HOME then out-and-about with zero usage", listed)
	}

	deleted := decodeTagDeletion(t, runJSON("tags", "delete", "out-and-about"))
	if deleted.Tag.ID != renamed.ID || deleted.Tag.Title != renamed.Title || deleted.Detached != 0 {
		t.Errorf("deleted tag = %#v, want out-and-about detached from zero items", deleted)
	}
	remaining := decodeListedTagRows(t, runJSON("tags", "list"))
	if len(remaining) != 1 || remaining[0].ID != caseRenamed.ID || remaining[0].Title != caseRenamed.Title {
		t.Errorf("tags after deletion = %#v, want only HOME", remaining)
	}

	decodeTagError(t, runJSON("tags", "rename", "ghost", "x"), apperr.NotFound)
	decodeTagError(t, runJSON("tags", "delete", "ghost"), apperr.NotFound)
	decodeTagError(t, runJSON("tags", "add", ""), apperr.InvalidArgument)
}

func decodeTagRow(t *testing.T, result processResult) tagRow {
	t.Helper()
	return decodeJSON[tagRow](t, result, "tag")
}

func decodeListedTagRows(t *testing.T, result processResult) []listedTagRow {
	t.Helper()
	return decodeJSON[[]listedTagRow](t, result, "listed tags")
}

func decodeTagDeletion(t *testing.T, result processResult) tagDeletion {
	t.Helper()
	return decodeJSON[tagDeletion](t, result, "tag deletion")
}

func decodeTagError(t *testing.T, result processResult, wantCode apperr.Code) string {
	t.Helper()
	assertJSONError(t, result, wantCode)

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode tag error: %v", err)
	}
	return envelope.Error.Message
}
