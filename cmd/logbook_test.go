package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/logbook"
)

type fakeLogbookApplication struct {
	result []logbook.Entry
	err    error
	calls  int
}

func (f *fakeLogbookApplication) List(context.Context) ([]logbook.Entry, error) {
	f.calls++
	return f.result, f.err
}

func runLogbookCommand(
	t *testing.T,
	application logbook.Application,
	location *time.Location,
	args ...string,
) commandResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := commandResult{}
	factory := func(_ context.Context, path string) (applications, io.Closer, error) {
		result.opens++
		result.openPath = path
		return applications{logbook: application}, closeRecorder{close: func() {
			result.closes++
		}}, nil
	}
	root := newRootCommandWithFactoryAndLocation(factory, location)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	result.exitCode = execute(root, args)
	result.stdout = stdout.String()
	result.stderr = stderr.String()

	return result
}

func TestLogbookJSONPreservesEntriesAndFactoryLifecycle(t *testing.T) {
	t.Parallel()

	projectTitle := "Release"
	areaID := int64(4)
	areaTitle := "Operations"
	entries := []logbook.Entry{
		{
			Kind:         "project",
			ID:           12,
			Title:        "Release",
			Status:       "done",
			ResolvedAt:   "2026-07-28T01:30:00.000Z",
			ProjectTitle: nil,
		},
		{
			Kind:               "task",
			ID:                 3,
			Title:              "Publish notes",
			Status:             "cancelled",
			ResolvedAt:         "2026-07-27T23:00:00.000Z",
			ProjectTitle:       &projectTitle,
			GoverningAreaID:    &areaID,
			GoverningAreaTitle: &areaTitle,
		},
	}
	application := &fakeLogbookApplication{result: entries}
	result := runLogbookCommand(
		t,
		application,
		time.UTC,
		"--db",
		"chosen.db",
		"--json",
		"logbook",
	)

	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want JSON success", result)
	}
	if !strings.HasSuffix(result.stdout, "\n") || strings.Count(result.stdout, "\n") != 1 {
		t.Errorf("stdout = %q, want one newline-terminated JSON value", result.stdout)
	}
	var decoded []logbook.Entry
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode logbook JSON: %v", err)
	}
	if !reflect.DeepEqual(decoded, entries) {
		t.Errorf("decoded entries = %#v, want %#v", decoded, entries)
	}
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &objects); err != nil {
		t.Fatalf("decode logbook JSON fields: %v", err)
	}
	for index, object := range objects {
		if len(object) != 8 {
			t.Errorf("entry %d field count = %d, want 8", index, len(object))
		}
		for _, field := range []string{
			"kind", "id", "title", "status", "resolved_at", "project_title",
			"governing_area_id", "governing_area_title",
		} {
			if _, ok := object[field]; !ok {
				t.Errorf("entry %d fields = %v, missing %q", index, object, field)
			}
		}
	}
	for _, field := range []string{"project_title", "governing_area_id", "governing_area_title"} {
		if string(objects[0][field]) != "null" {
			t.Errorf("project %s = %s, want null", field, objects[0][field])
		}
	}
	if application.calls != 1 || result.openPath != "chosen.db" || result.opens != 1 || result.closes != 1 {
		t.Errorf("call/factory lifecycle = %d/%#v, want one call and chosen path with one open/close", application.calls, result)
	}
}

func TestLogbookHumanOutputPreservesOrderAndUsesLocalCalendarDay(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("UTC-08", -8*60*60)
	areaID := int64(9)
	areaTitle := "Ignored in human output"
	entries := []logbook.Entry{
		{
			Kind:       "project",
			ID:         12,
			Title:      "First",
			Status:     "cancelled",
			ResolvedAt: "2026-07-28T01:30:00Z",
		},
		{
			Kind:               "task",
			ID:                 3,
			Title:              "Second",
			Status:             "done",
			ResolvedAt:         "2026-07-28T09:30:00Z",
			GoverningAreaID:    &areaID,
			GoverningAreaTitle: &areaTitle,
		},
	}
	result := runLogbookCommand(
		t,
		&fakeLogbookApplication{result: entries},
		location,
		"logbook",
	)

	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want human success", result)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) != 2 ||
		strings.Join(strings.Fields(lines[0]), " ") != "project 12 First cancelled 2026-07-27" ||
		strings.Join(strings.Fields(lines[1]), " ") != "task 3 Second done 2026-07-28" {
		t.Errorf("stdout = %q, want ordered local-day rows", result.stdout)
	}
	if strings.Contains(result.stdout, "\x1b[") {
		t.Errorf("stdout = %q, want no ANSI", result.stdout)
	}
}

func TestLogbookEmptyOutputAndNoArguments(t *testing.T) {
	t.Parallel()

	human := runLogbookCommand(
		t,
		&fakeLogbookApplication{result: []logbook.Entry{}},
		time.UTC,
		"logbook",
	)
	if human.exitCode != 0 || human.stdout != "" || human.stderr != "" {
		t.Errorf("empty human result = %#v, want no output", human)
	}

	jsonResult := runLogbookCommand(
		t,
		&fakeLogbookApplication{result: []logbook.Entry{}},
		time.UTC,
		"logbook",
		"--json",
	)
	if jsonResult.exitCode != 0 || jsonResult.stdout != "[]\n" || jsonResult.stderr != "" {
		t.Errorf("empty JSON result = %#v, want compact empty array", jsonResult)
	}

	invalid := runLogbookCommand(t, &fakeLogbookApplication{}, time.UTC, "logbook", "extra")
	if invalid.exitCode != 2 || invalid.opens != 0 || invalid.stdout != "" || invalid.stderr == "" {
		t.Errorf("argument result = %#v, want stderr-only usage failure without opening", invalid)
	}
}

func TestLogbookHumanOutputEscapesStoredControls(t *testing.T) {
	t.Parallel()

	entries := []logbook.Entry{{
		Kind:       "ta\x1bsk",
		ID:         1,
		Title:      "safe\rforged\nrow",
		Status:     "done\a",
		ResolvedAt: "2026-07-28T12:00:00Z",
	}}
	result := runLogbookCommand(
		t,
		&fakeLogbookApplication{result: entries},
		time.UTC,
		"logbook",
	)

	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want human success", result)
	}
	if strings.ContainsAny(result.stdout, "\x1b\r\a") {
		t.Errorf("stdout = %q, want terminal controls escaped", result.stdout)
	}
	for _, visible := range []string{`ta\x1bsk`, `safe\rforged\nrow`, `done\a`} {
		if !strings.Contains(result.stdout, visible) {
			t.Errorf("stdout = %q, want visible control escape %q", result.stdout, visible)
		}
	}
}

func TestLogbookInvalidTimestampBecomesInternalErrorOnStderr(t *testing.T) {
	t.Parallel()

	application := &fakeLogbookApplication{result: []logbook.Entry{{
		Kind:       "task",
		ID:         7,
		Title:      "Broken",
		Status:     "done",
		ResolvedAt: "not-a-timestamp",
	}}}
	result := runLogbookCommand(t, application, time.UTC, "logbook")

	if result.exitCode != 1 || result.stdout != "" {
		t.Errorf("result = %#v, want stderr-only normalized application error", result)
	}
	if !strings.Contains(result.stderr, "Error: parse logbook resolved_at for task 7:") {
		t.Errorf("stderr = %q, want invalid stored timestamp diagnostic", result.stderr)
	}
	if application.calls != 1 || result.opens != 1 || result.closes != 1 {
		t.Errorf("call/factory lifecycle = %d/%#v, want one call, open, and close", application.calls, result)
	}
}
