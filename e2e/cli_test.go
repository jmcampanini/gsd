package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/task"
)

var (
	binaryPath      string
	expectedVersion string
	workDir         string
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	repositoryRoot, err := filepath.Abs("..")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repository root: %v\n", err)
		return 1
	}

	sandbox := filepath.Join(repositoryRoot, ".sandbox")
	if err := os.MkdirAll(sandbox, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create sandbox: %v\n", err)
		return 1
	}

	workDir, err = os.MkdirTemp(sandbox, "e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create e2e work directory: %v\n", err)
		return 1
	}
	defer func() {
		_ = os.RemoveAll(workDir)
	}()

	binaryPath = filepath.Join(workDir, "gsd")
	expectedVersion = gitVersion(repositoryRoot)
	build := exec.Command("make", "BUILD_DIR="+workDir, "build")
	build.Dir = repositoryRoot
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build e2e binary: %v\n", err)
		return 1
	}

	return m.Run()
}

func gitVersion(repositoryRoot string) string {
	command := exec.Command("git", "describe", "--tags", "--dirty", "--always")
	command.Dir = repositoryRoot
	output, err := command.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

type processResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func runGSD(t *testing.T, args ...string) processResult {
	t.Helper()
	return runGSDWithEnv(t, nil, args...)
}

func runGSDWithEnv(t *testing.T, environment map[string]string, args ...string) processResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.Command(binaryPath, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.Env = filteredEnvironment(environment)

	exitCode := 0
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("run gsd: %v", err)
		}
		exitCode = exitError.ExitCode()
	}

	return processResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

func filteredEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrides[key]; !overridden {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}

	return environment
}

func TestVersion(t *testing.T) {
	t.Parallel()

	result := runGSD(t, "--version")
	if result.exitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.exitCode)
	}
	if result.stdout != "gsd version "+expectedVersion+"\n" {
		t.Errorf("stdout = %q, want version output", result.stdout)
	}
	if result.stderr != "" {
		t.Errorf("stderr = %q, want empty", result.stderr)
	}
}

func TestHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "bare root"},
		{name: "help flag", args: []string{"--help"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runGSD(t, test.args...)
			if result.exitCode != 0 {
				t.Errorf("exit code = %d, want 0", result.exitCode)
			}
			if !strings.Contains(result.stdout, "Usage:\n  gsd [flags]") {
				t.Errorf("stdout = %q, want help usage", result.stdout)
			}
			if result.stderr != "" {
				t.Errorf("stderr = %q, want empty", result.stderr)
			}
		})
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()

	result := runGSD(t, "nonsense")
	if result.exitCode != 2 {
		t.Errorf("exit code = %d, want 2", result.exitCode)
	}
	if result.stdout != "" {
		t.Errorf("stdout = %q, want empty", result.stdout)
	}
	if !strings.Contains(result.stderr, "unknown command \"nonsense\" for \"gsd\"") {
		t.Errorf("stderr = %q, want unknown-command diagnostic", result.stderr)
	}
}

func TestParseError(t *testing.T) {
	t.Parallel()

	result := runGSD(t, "--unknown")
	if result.exitCode != 2 {
		t.Errorf("exit code = %d, want 2", result.exitCode)
	}
	if result.stdout != "" {
		t.Errorf("stdout = %q, want empty", result.stdout)
	}
	if !strings.Contains(result.stderr, "Error: unknown flag: --unknown") {
		t.Errorf("stderr = %q, want parse diagnostic", result.stderr)
	}
}

func TestTaskWorkflow(t *testing.T) {
	t.Parallel()

	workflowDir, err := os.MkdirTemp(workDir, "capture-")
	if err != nil {
		t.Fatalf("create workflow directory: %v", err)
	}
	databasePath := filepath.Join(workflowDir, "nested", "gsd.db")
	otherDatabasePath := filepath.Join(workflowDir, "other.db")

	firstResult := runGSDWithEnv(
		t,
		map[string]string{"GSD_DB": otherDatabasePath},
		"add",
		"first",
		"--note",
		"details",
		"--db",
		databasePath,
		"--json",
	)
	first := decodeTask(t, firstResult)
	if first.ID != 1 || first.Title != "first" || first.Note != "details" || first.Status != "open" {
		t.Errorf("first task = %#v, want captured open task", first)
	}
	if _, err := os.Stat(otherDatabasePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("explicit --db did not override GSD_DB; other database stat error = %v", err)
	}

	secondResult := runGSDWithEnv(
		t,
		map[string]string{"GSD_DB": databasePath},
		"add",
		"second",
		"--json",
	)
	second := decodeTask(t, secondResult)
	if second.ID != 2 || second.Position != 1 {
		t.Errorf("second task = %#v, want ID 2 at position 1", second)
	}

	third := decodeTask(t, runGSD(t, "add", "third", "--db", databasePath, "--json"))
	if third.ID != 3 || third.Position != 2 {
		t.Errorf("third task = %#v, want ID 3 at position 2", third)
	}

	inboxResult := runGSD(t, "inbox", "--db", databasePath, "--json")
	inbox := decodeTasks(t, inboxResult)
	if len(inbox) != 3 || inbox[0].ID != first.ID || inbox[1].ID != second.ID || inbox[2].ID != third.ID {
		t.Errorf("inbox = %#v, want three tasks in position order", inbox)
	}
	if strings.Contains(inboxResult.stdout, "\x1b[") {
		t.Errorf("inbox JSON = %q, want no terminal control sequences", inboxResult.stdout)
	}

	shownResult := runGSD(t, "show", "2", "--db", databasePath, "--json")
	shown := decodeTask(t, shownResult)
	if shown != second {
		t.Errorf("shown task = %#v, want %#v", shown, second)
	}

	done := decodeTask(t, runGSD(t, "done", "1", "--db", databasePath, "--json"))
	if done.Status != "done" || done.DoneAt == nil || done.CancelledAt != nil || done.Position != first.Position {
		t.Errorf("done task = %#v, want completed first task with preserved position", done)
	}
	cancelled := decodeTask(t, runGSD(t, "cancel", "2", "--db", databasePath, "--json"))
	if cancelled.Status != "cancelled" || cancelled.CancelledAt == nil || cancelled.DoneAt != nil || cancelled.Position != second.Position {
		t.Errorf("cancelled task = %#v, want cancelled second task with preserved position", cancelled)
	}

	remaining := decodeTasks(t, runGSD(t, "inbox", "--db", databasePath, "--json"))
	if len(remaining) != 1 || remaining[0].ID != third.ID {
		t.Errorf("remaining inbox = %#v, want only third task", remaining)
	}
	all := decodeTasks(t, runGSD(t, "list", "--status", "all", "--db", databasePath, "--json"))
	if len(all) != 3 || all[0].Status != "done" || all[1].Status != "cancelled" || all[2].Status != "open" {
		t.Errorf("all tasks = %#v, want done, cancelled, and open in position order", all)
	}
	doneTasks := decodeTasks(t, runGSD(t, "list", "--status", "done", "--db", databasePath, "--json"))
	if len(doneTasks) != 1 || doneTasks[0].ID != first.ID {
		t.Errorf("done tasks = %#v, want only first task", doneTasks)
	}

	reopened := decodeTask(t, runGSD(t, "reopen", "1", "--db", databasePath, "--json"))
	if reopened.Status != "open" || reopened.DoneAt != nil || reopened.CancelledAt != nil {
		t.Errorf("reopened task = %#v, want open task", reopened)
	}
	redone := decodeTask(t, runGSD(t, "done", "1", "--db", databasePath, "--json"))
	if redone.Status != "done" || redone.DoneAt == nil {
		t.Errorf("completed reopened task = %#v, want done task", redone)
	}
	repeatedDone := runGSD(t, "done", "1", "--db", databasePath, "--json")
	assertJSONError(t, repeatedDone, task.ErrorConflict)

	deleted := decodeTask(t, runGSD(t, "delete", "2", "--db", databasePath, "--json"))
	if !reflect.DeepEqual(deleted, cancelled) {
		t.Errorf("deleted task = %#v, want snapshot %#v", deleted, cancelled)
	}
	assertJSONError(
		t,
		runGSD(t, "show", "2", "--db", databasePath, "--json"),
		task.ErrorNotFound,
	)

	assertJSONError(
		t,
		runGSD(t, "show", "99", "--db", databasePath, "--json"),
		task.ErrorNotFound,
	)

	emptyResult := runGSD(t, "inbox", "--db", filepath.Join(workflowDir, "empty.db"))
	if emptyResult.exitCode != 0 || emptyResult.stdout != "" || emptyResult.stderr != "" {
		t.Errorf("empty human inbox = %#v, want no output", emptyResult)
	}

	blockedPath := filepath.Join(workflowDir, "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}
	for _, args := range [][]string{
		{"--db", filepath.Join(blockedPath, "gsd.db"), "--help"},
		{"--db", filepath.Join(blockedPath, "gsd.db"), "--version"},
	} {
		result := runGSD(t, args...)
		if result.exitCode != 0 || result.stderr != "" {
			t.Errorf("informational command %v = %#v, want success without database open", args, result)
		}
	}
}

func decodeTask(t *testing.T, result processResult) task.Task {
	t.Helper()
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("command result = %#v, want JSON success", result)
	}
	if !strings.HasSuffix(result.stdout, "\n") || strings.Count(result.stdout, "\n") != 1 {
		t.Fatalf("stdout = %q, want one newline-terminated JSON value", result.stdout)
	}

	var decoded task.Task
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	return decoded
}

func decodeTasks(t *testing.T, result processResult) []task.Task {
	t.Helper()
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("command result = %#v, want JSON success", result)
	}
	if !strings.HasSuffix(result.stdout, "\n") || strings.Count(result.stdout, "\n") != 1 {
		t.Fatalf("stdout = %q, want one newline-terminated JSON value", result.stdout)
	}

	var decoded []task.Task
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	return decoded
}

func assertJSONError(t *testing.T, result processResult, wantCode task.ErrorCode) {
	t.Helper()
	if result.exitCode != 1 || result.stdout != "" {
		t.Fatalf("command result = %#v, want stderr-only exit 1", result)
	}
	if !strings.HasSuffix(result.stderr, "\n") || strings.Count(result.stderr, "\n") != 1 {
		t.Fatalf("stderr = %q, want one newline-terminated JSON value", result.stderr)
	}

	var envelope struct {
		Error struct {
			Code task.ErrorCode `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q", envelope.Error.Code, wantCode)
	}
}
