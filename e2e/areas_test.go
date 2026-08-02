package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type areaRow struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Note       string  `json:"note"`
	ArchivedAt *string `json:"archived_at"`
	Position   int64   `json:"position"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

func TestAreaCRUDWorkflow(t *testing.T) {
	databasePath := filepath.Join(workDir, "areas", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	home := decodeAreaRow(t, runJSON("areas", "add", "Home"))
	health := decodeAreaRow(t, runJSON("areas", "add", "Health"))
	if home.ID != 1 || home.Title != "Home" || home.Note != "" || home.ArchivedAt != nil ||
		home.Position != 0 || home.CreatedAt == "" || home.UpdatedAt == "" {
		t.Errorf("home area = %#v, want complete first active area row", home)
	}
	if health.ID != 2 || health.Title != "Health" || health.Note != "" ||
		health.ArchivedAt != nil || health.Position != 1 || health.CreatedAt == "" ||
		health.UpdatedAt == "" {
		t.Errorf("health area = %#v, want complete second active area row", health)
	}

	listed := decodeAreaRows(t, runJSON("areas", "list"))
	if len(listed) != 2 || listed[0].ID != home.ID || listed[1].ID != health.ID {
		t.Fatalf("listed areas = %#v, want Home then Health", listed)
	}
	if !reflect.DeepEqual(listed[0], home) || !reflect.DeepEqual(listed[1], health) {
		t.Errorf("listed areas = %#v, want persisted creation rows %#v", listed, []areaRow{home, health})
	}

	shownHome := decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(home.ID)))
	if !reflect.DeepEqual(shownHome, home) {
		t.Errorf("shown Home = %#v, want %#v", shownHome, home)
	}

	editedHome := decodeAreaRow(t, runJSON(
		"area",
		"edit",
		fmt.Sprint(home.ID),
		"--note",
		"Everything house",
	))
	persistedHome := decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(home.ID)))
	if editedHome.Note != "Everything house" || !reflect.DeepEqual(persistedHome, editedHome) {
		t.Errorf("edited/shown Home = %#v/%#v, want persisted note", editedHome, persistedHome)
	}

	assertJSONError(t, runJSON("areas", "add", ""), apperr.InvalidArgument)
	assertJSONError(t, runJSON("area", "show", "99"), apperr.NotFound)

	for index, noun := range []string{"areas", "area"} {
		unusedPath := filepath.Join(workDir, fmt.Sprintf("bare-area-%d.db", index))
		result := runGSD(t, noun, "--db", unusedPath)
		if result.exitCode != 2 || result.stdout != "" || result.stderr == "" {
			t.Errorf("bare %s = %#v, want stderr-only usage error", noun, result)
		}
		if _, err := os.Stat(unusedPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("bare %s database stat error = %v, want not exist", noun, err)
		}
	}
}

func decodeAreaRow(t *testing.T, result processResult) areaRow {
	t.Helper()
	decoded := decodeJSON[areaRow](t, result, "area")
	assertAreaObjectShape(t, []byte(result.stdout))
	return decoded
}

func decodeAreaRows(t *testing.T, result processResult) []areaRow {
	t.Helper()
	decoded := decodeJSON[[]areaRow](t, result, "areas")

	var objects []json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &objects); err != nil {
		t.Fatalf("decode area objects: %v", err)
	}
	for _, object := range objects {
		assertAreaObjectShape(t, object)
	}
	return decoded
}

func assertAreaObjectShape(t *testing.T, data []byte) {
	t.Helper()

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("decode area object: %v", err)
	}
	expected := []string{
		"id",
		"title",
		"note",
		"archived_at",
		"position",
		"created_at",
		"updated_at",
	}
	if len(object) != len(expected) {
		t.Fatalf("area JSON fields = %v, want exactly %v", object, expected)
	}
	for _, field := range expected {
		if _, exists := object[field]; !exists {
			t.Fatalf("area JSON fields = %v, want field %q", object, field)
		}
	}
}
