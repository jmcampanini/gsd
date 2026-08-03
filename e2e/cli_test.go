package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
	_ "modernc.org/sqlite"
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
	return runGSDProcess(t, nil, "", args...)
}

func runGSDWithEnv(t *testing.T, environment map[string]string, args ...string) processResult {
	t.Helper()
	return runGSDProcess(t, environment, "", args...)
}

func runGSDWithInput(t *testing.T, input string, args ...string) processResult {
	t.Helper()
	return runGSDProcess(t, nil, input, args...)
}

func runGSDProcess(
	t *testing.T,
	environment map[string]string,
	input string,
	args ...string,
) processResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.Command(binaryPath, args...)
	command.Stdin = strings.NewReader(input)
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
		switch key {
		case "GSD_DB", "XDG_CONFIG_HOME", "XDG_DATA_HOME":
			continue
		}
		if _, overridden := overrides[key]; !overridden {
			environment = append(environment, entry)
		}
	}
	if _, overridden := overrides["XDG_CONFIG_HOME"]; !overridden {
		environment = append(environment, "XDG_CONFIG_HOME="+filepath.Join(workDir, "environment", "config"))
	}
	if _, overridden := overrides["XDG_DATA_HOME"]; !overridden {
		environment = append(environment, "XDG_DATA_HOME="+filepath.Join(workDir, "environment", "data"))
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}

	return environment
}

func TestVersion(t *testing.T) {
	t.Parallel()

	result := runGSD(t, "--config", "/nonexistent.toml", "--version")
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
		{name: "help flag", args: []string{"--config", "/nonexistent.toml", "--help"}},
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

	result := runGSD(t, "--config", "/nonexistent.toml", "--unknown")
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

func TestDatabaseConfigPrecedenceAndFailures(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp(workDir, "config-")
	if err != nil {
		t.Fatalf("create config workflow directory: %v", err)
	}
	configHome := filepath.Join(dir, "config-home")
	dataHome := filepath.Join(dir, "data-home")
	configPath := filepath.Join(configHome, "gsd", "config.toml")
	writeE2EConfig(t, configPath, "db_path = 'configured.db'\n")
	baseEnvironment := map[string]string{
		"XDG_CONFIG_HOME": configHome,
		"XDG_DATA_HOME":   dataHome,
	}

	fromFile := decodeTask(t, runGSDWithEnv(t, baseEnvironment, "add", "from file", "--json"))
	fileDatabase := filepath.Join(configHome, "gsd", "configured.db")
	if fromFile.ID != 1 {
		t.Errorf("file task ID = %d, want 1 in configured database", fromFile.ID)
	}
	if _, err := os.Stat(fileDatabase); err != nil {
		t.Errorf("configured database stat error = %v", err)
	}

	environmentDatabase := filepath.Join(dir, "environment.db")
	environment := map[string]string{
		"XDG_CONFIG_HOME": configHome,
		"XDG_DATA_HOME":   dataHome,
		"GSD_DB":          environmentDatabase,
	}
	fromEnvironment := decodeTask(t, runGSDWithEnv(t, environment, "add", "from environment", "--json"))
	if fromEnvironment.ID != 1 {
		t.Errorf("environment task ID = %d, want 1 in overridden database", fromEnvironment.ID)
	}
	if _, err := os.Stat(environmentDatabase); err != nil {
		t.Errorf("environment database stat error = %v", err)
	}

	flagDatabase := filepath.Join(dir, "flag.db")
	fromFlag := decodeTask(t, runGSDWithEnv(
		t,
		environment,
		"add",
		"from flag",
		"--db",
		flagDatabase,
		"--json",
	))
	if fromFlag.ID != 1 {
		t.Errorf("flag task ID = %d, want 1 in flag database", fromFlag.ID)
	}
	if _, err := os.Stat(flagDatabase); err != nil {
		t.Errorf("flag database stat error = %v", err)
	}

	explicitConfig := filepath.Join(dir, "explicit", "config.toml")
	writeE2EConfig(t, explicitConfig, "db_path = 'explicit.db'\n")
	fromExplicit := decodeTask(t, runGSDWithEnv(
		t,
		baseEnvironment,
		"add",
		"from explicit file",
		"--config",
		explicitConfig,
		"--json",
	))
	if fromExplicit.ID != 1 {
		t.Errorf("explicit task ID = %d, want 1 in explicit database", fromExplicit.ID)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(explicitConfig), "explicit.db")); err != nil {
		t.Errorf("explicit database stat error = %v", err)
	}

	assertJSONError(
		t,
		runGSDWithEnv(t, baseEnvironment, "inbox", "--config", filepath.Join(dir, "missing.toml"), "--json"),
		apperr.InvalidArgument,
	)

	absentConfigHome := filepath.Join(dir, "absent-config")
	absentDataHome := filepath.Join(dir, "absent-data")
	absent := decodeTasks(t, runGSDWithEnv(t, map[string]string{
		"XDG_CONFIG_HOME": absentConfigHome,
		"XDG_DATA_HOME":   absentDataHome,
	}, "inbox", "--json"))
	if len(absent) != 0 {
		t.Errorf("absent discovered config inbox = %#v, want empty", absent)
	}
	if _, err := os.Stat(filepath.Join(absentDataHome, "gsd", "gsd.db")); err != nil {
		t.Errorf("default database stat error = %v", err)
	}
}

func TestConfigReportRoundTripsWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp(workDir, "config-report-")
	if err != nil {
		t.Fatalf("create config report workflow directory: %v", err)
	}
	configPath := filepath.Join(dir, "original", "config.toml")
	writeE2EConfig(t, configPath, "db_path = 'nested/gsd.db'\n")
	environment := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(dir, "config-home"),
		"XDG_DATA_HOME":   filepath.Join(dir, "data-home"),
	}

	first := runGSDWithEnv(t, environment, "config", "--config", configPath)
	wantDatabase := filepath.Join(filepath.Dir(configPath), "nested", "gsd.db")
	wantReport := fmt.Sprintf("db_path = %q\n", wantDatabase)
	if first.exitCode != 0 || first.stdout != wantReport || first.stderr != "" {
		t.Fatalf("first config report = %#v, want %q", first, wantReport)
	}
	relativeEnvironmentDatabase := filepath.Join("relative", filepath.Base(dir), "environment.db")
	environmentDatabase, err := filepath.Abs(relativeEnvironmentDatabase)
	if err != nil {
		t.Fatalf("resolve expected environment database: %v", err)
	}
	withEnvironment := map[string]string{
		"XDG_CONFIG_HOME": environment["XDG_CONFIG_HOME"],
		"XDG_DATA_HOME":   environment["XDG_DATA_HOME"],
		"GSD_DB":          relativeEnvironmentDatabase,
	}
	provenance := runGSDWithEnv(
		t,
		withEnvironment,
		"config",
		"--config",
		configPath,
		"--provenance",
	)
	wantProvenance := fmt.Sprintf("db_path = %q # env: GSD_DB\n", environmentDatabase)
	if provenance.exitCode != 0 || provenance.stdout != wantProvenance || provenance.stderr != "" {
		t.Errorf("provenance report = %#v, want %q", provenance, wantProvenance)
	}
	environmentSnapshotPath := filepath.Join(dir, "environment-snapshot", "config.toml")
	writeE2EConfig(t, environmentSnapshotPath, provenance.stdout)
	environmentRoundTrip := runGSDWithEnv(t, environment, "config", "--config", environmentSnapshotPath)
	wantEnvironmentReport := fmt.Sprintf("db_path = %q\n", environmentDatabase)
	if environmentRoundTrip.exitCode != 0 || environmentRoundTrip.stdout != wantEnvironmentReport ||
		environmentRoundTrip.stderr != "" {
		t.Errorf("relative environment round-trip = %#v, want %q", environmentRoundTrip, wantEnvironmentReport)
	}

	snapshotPath := filepath.Join(dir, "snapshot", "config.toml")
	writeE2EConfig(t, snapshotPath, first.stdout)
	roundTrip := runGSDWithEnv(t, environment, "config", "--config", snapshotPath)
	if roundTrip.exitCode != 0 || roundTrip.stdout != first.stdout || roundTrip.stderr != "" {
		t.Errorf("round-trip report = %#v, want identical output %q", roundTrip, first.stdout)
	}

	missingPath := filepath.Join(dir, "missing.toml")
	missing := runGSDWithEnv(t, environment, "config", "--config", missingPath)
	if missing.exitCode != 1 || missing.stdout != "" || !strings.Contains(missing.stderr, "invalid configuration") {
		t.Errorf("missing explicit config = %#v, want fail-loud application error", missing)
	}

	unsupportedJSON := runGSDWithEnv(
		t,
		environment,
		"config",
		"--config",
		missingPath,
		"--json",
	)
	if unsupportedJSON.exitCode != 2 || unsupportedJSON.stdout != "" ||
		!strings.Contains(unsupportedJSON.stderr, "--json is not supported by gsd config") {
		t.Errorf("config JSON = %#v, want usage error before loading config", unsupportedJSON)
	}

	for _, databasePath := range []string{wantDatabase, environmentDatabase} {
		if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("reported database %q stat error = %v, want not created", databasePath, err)
		}
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
	if !reflect.DeepEqual(shown, second) {
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
	assertJSONError(t, repeatedDone, apperr.Conflict)

	editedNote := "line one\nline two\n"
	edited := decodeTask(t, runGSDWithInput(
		t,
		editedNote,
		"edit",
		"3",
		"--title",
		"  revised third  ",
		"--note",
		"-",
		"--db",
		databasePath,
		"--json",
	))
	if edited.Title != "  revised third  " || edited.Note != editedNote {
		t.Errorf("edited task = %#v, want exact title and note", edited)
	}
	shownEdited := decodeTask(t, runGSD(t, "show", "3", "--db", databasePath, "--json"))
	if !reflect.DeepEqual(shownEdited, edited) {
		t.Errorf("shown edited task = %#v, want %#v", shownEdited, edited)
	}
	assertJSONError(
		t,
		runGSD(t, "edit", "3", "--db", databasePath, "--json"),
		apperr.InvalidArgument,
	)

	deleted := decodeTask(t, runGSD(t, "delete", "2", "--db", databasePath, "--json"))
	if !reflect.DeepEqual(deleted, cancelled) {
		t.Errorf("deleted task = %#v, want snapshot %#v", deleted, cancelled)
	}
	assertJSONError(
		t,
		runGSD(t, "show", "2", "--db", databasePath, "--json"),
		apperr.NotFound,
	)

	assertJSONError(
		t,
		runGSD(t, "show", "99", "--db", databasePath, "--json"),
		apperr.NotFound,
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
		{"--config", "/nonexistent.toml", "--db", filepath.Join(blockedPath, "gsd.db"), "--help"},
		{"--config", "/nonexistent.toml", "--db", filepath.Join(blockedPath, "gsd.db"), "--version"},
	} {
		result := runGSD(t, args...)
		if result.exitCode != 0 || result.stderr != "" {
			t.Errorf("informational command %v = %#v, want success without database open", args, result)
		}
	}

	wrongRevisionPath := filepath.Join(workflowDir, "wrong-revision.db")
	wrongRevisionDatabase, err := sql.Open("sqlite", wrongRevisionPath)
	if err != nil {
		t.Fatalf("open wrong-revision database: %v", err)
	}
	if _, err := wrongRevisionDatabase.Exec("PRAGMA user_version = 42"); err != nil {
		_ = wrongRevisionDatabase.Close()
		t.Fatalf("set wrong database revision: %v", err)
	}
	if err := wrongRevisionDatabase.Close(); err != nil {
		t.Fatalf("close wrong-revision database: %v", err)
	}
	assertJSONError(
		t,
		runGSD(t, "inbox", "--db", wrongRevisionPath, "--json"),
		apperr.Conflict,
	)
}

func TestTaskTimeWorkflow(t *testing.T) {
	for attempt := range 3 {
		reference := time.Now()
		capturedDate := calendarDate(reference, 0)
		tomorrow := calendarDate(reference, 1)
		yesterday := calendarDate(reference, -1)
		weekLater := calendarDate(reference, 7)
		databasePath := filepath.Join(workDir, fmt.Sprintf("time-%d.db", attempt))

		created := decodeTask(t, runGSD(
			t,
			"add",
			"deadline",
			"--due",
			"tomorrow",
			"--db",
			databasePath,
			"--json",
		))
		shown := decodeTask(t, runGSD(t, "show", fmt.Sprint(created.ID), "--db", databasePath, "--json"))
		due := decodeTasks(t, runGSD(t, "list", "--due", "--db", databasePath, "--json"))
		initialOverdue := decodeTasks(t, runGSD(t, "list", "--overdue", "--db", databasePath, "--json"))
		humanShow := runGSD(t, "show", fmt.Sprint(created.ID), "--db", databasePath)
		humanList := runGSD(t, "list", "--due", "--db", databasePath)

		edited := decodeTask(t, runGSD(
			t,
			"edit",
			fmt.Sprint(created.ID),
			"--due",
			yesterday,
			"--db",
			databasePath,
			"--json",
		))
		overdue := decodeTasks(t, runGSD(t, "list", "--overdue", "--db", databasePath, "--json"))
		decodeTask(t, runGSD(t, "done", fmt.Sprint(created.ID), "--db", databasePath, "--json"))
		openOverdue := decodeTasks(t, runGSD(t, "list", "--overdue", "--db", databasePath, "--json"))
		doneOverdue := decodeTasks(t, runGSD(
			t,
			"list",
			"--status",
			"done",
			"--overdue",
			"--db",
			databasePath,
			"--json",
		))
		decodeTask(t, runGSD(t, "reopen", fmt.Sprint(created.ID), "--db", databasePath, "--json"))
		cleared := decodeTask(t, runGSD(
			t,
			"edit",
			fmt.Sprint(created.ID),
			"--no-due",
			"--db",
			databasePath,
			"--json",
		))
		dueAfterClear := decodeTasks(t, runGSD(t, "list", "--due", "--db", databasePath, "--json"))

		undated := decodeTask(t, runGSD(t, "add", "undated", "--db", databasePath, "--json"))
		todayDeferred := decodeTask(t, runGSD(
			t,
			"add",
			"today deferred",
			"--defer",
			"today",
			"--db",
			databasePath,
			"--json",
		))
		tomorrowDeferred := decodeTask(t, runGSD(
			t,
			"add",
			"tomorrow deferred",
			"--defer",
			"tomorrow",
			"--due",
			"today",
			"--db",
			databasePath,
			"--json",
		))
		weekDeferred := decodeTask(t, runGSD(
			t,
			"add",
			"week deferred",
			"--defer",
			"+1w",
			"--db",
			databasePath,
			"--json",
		))
		dueToday := decodeTask(t, runGSD(
			t,
			"add",
			"due today",
			"--due",
			"today",
			"--db",
			databasePath,
			"--json",
		))

		available := decodeTasks(t, runGSD(t, "available", "--db", databasePath, "--json"))
		deferred := decodeTasks(t, runGSD(t, "list", "--deferred", "--db", databasePath, "--json"))
		humanDeferred := runGSD(t, "show", fmt.Sprint(tomorrowDeferred.ID), "--db", databasePath)
		humanAvailable := runGSD(t, "available", "--db", databasePath)
		clearedDefer := decodeTask(t, runGSD(
			t,
			"edit",
			fmt.Sprint(tomorrowDeferred.ID),
			"--no-defer",
			"--db",
			databasePath,
			"--json",
		))
		availableAfterClear := decodeTasks(t, runGSD(t, "available", "--db", databasePath, "--json"))
		deferredAfterClear := decodeTasks(t, runGSD(t, "list", "--deferred", "--db", databasePath, "--json"))
		decodeTask(t, runGSD(t, "done", fmt.Sprint(weekDeferred.ID), "--db", databasePath, "--json"))
		openDeferred := decodeTasks(t, runGSD(t, "list", "--deferred", "--db", databasePath, "--json"))
		doneDeferred := decodeTasks(t, runGSD(
			t,
			"list",
			"--status",
			"done",
			"--deferred",
			"--db",
			databasePath,
			"--json",
		))
		overdueBeforeAdd := decodeTasks(t, runGSD(t, "list", "--overdue", "--db", databasePath, "--json"))
		addedOverdue := decodeTask(t, runGSD(
			t,
			"add",
			"added overdue",
			"--due",
			yesterday,
			"--db",
			databasePath,
			"--json",
		))
		addedOverdueList := decodeTasks(t, runGSD(t, "list", "--overdue", "--db", databasePath, "--json"))
		decodeTask(t, runGSD(t, "done", fmt.Sprint(addedOverdue.ID), "--db", databasePath, "--json"))
		addedOpenOverdue := decodeTasks(t, runGSD(t, "list", "--overdue", "--db", databasePath, "--json"))
		addedDoneOverdue := decodeTasks(t, runGSD(
			t,
			"list",
			"--status",
			"done",
			"--overdue",
			"--db",
			databasePath,
			"--json",
		))

		weekdayCases := []struct {
			token   string
			weekday time.Weekday
		}{
			{token: "mon", weekday: time.Monday},
			{token: "tue", weekday: time.Tuesday},
			{token: "wed", weekday: time.Wednesday},
			{token: "thu", weekday: time.Thursday},
			{token: "fri", weekday: time.Friday},
			{token: "sat", weekday: time.Saturday},
			{token: "sun", weekday: time.Sunday},
		}
		weekdayDeferred := make([]task.Task, len(weekdayCases))
		weekdayDates := make([]string, len(weekdayCases))
		for index, test := range weekdayCases {
			days := (int(test.weekday) - int(reference.Weekday()) + 7) % 7
			if days == 0 {
				days = 7
			}
			weekdayDates[index] = calendarDate(reference, days)
			weekdayDeferred[index] = decodeTask(t, runGSD(
				t,
				"add",
				"weekday "+test.token,
				"--defer",
				test.token,
				"--db",
				databasePath,
				"--json",
			))
		}

		for _, value := range []string{"2026-02-30", "2026-8-3", "next tuesday", "+3x"} {
			for _, flag := range []string{"--due", "--defer"} {
				assertJSONError(
					t,
					runGSD(t, "add", "invalid", flag, value, "--db", databasePath, "--json"),
					apperr.InvalidArgument,
				)
			}
		}

		selectorPairs := [][]string{
			{"--due", "--overdue"},
			{"--due", "--deferred"},
			{"--overdue", "--deferred"},
		}
		for index, selectors := range selectorPairs {
			conflictPath := filepath.Join(workDir, fmt.Sprintf("time-conflict-%d-%d.db", attempt, index))
			args := append([]string{"list"}, selectors...)
			args = append(args, "--db", conflictPath, "--json")
			conflict := runGSD(t, args...)
			if conflict.exitCode != 2 || conflict.stdout != "" || conflict.stderr == "" {
				t.Errorf("conflicting selectors %v = %#v, want stderr-only usage error", selectors, conflict)
			}
			if _, err := os.Stat(conflictPath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("conflicting selectors %v database stat error = %v, want not exist", selectors, err)
			}
		}

		if calendarDate(time.Now(), 0) != capturedDate {
			continue
		}

		if created.DueOn == nil || *created.DueOn != tomorrow ||
			shown.DueOn == nil || *shown.DueOn != tomorrow {
			t.Errorf("created/shown due dates = %#v/%#v, want %s", created.DueOn, shown.DueOn, tomorrow)
		}
		if len(due) != 1 || due[0].ID != created.ID || len(initialOverdue) != 0 {
			t.Errorf("initial due/overdue = %#v/%#v, want created task and empty overdue", due, initialOverdue)
		}
		if humanShow.exitCode != 0 || humanShow.stderr != "" ||
			!strings.Contains(humanShow.stdout, "Due on") ||
			!strings.Contains(humanShow.stdout, tomorrow) ||
			strings.Contains(humanShow.stdout, "\x1b[") {
			t.Errorf("human show = %#v, want plain labeled due date", humanShow)
		}
		if humanList.exitCode != 0 || humanList.stderr != "" ||
			!strings.Contains(humanList.stdout, "due "+tomorrow) ||
			strings.Contains(humanList.stdout, "\x1b[") {
			t.Errorf("human list = %#v, want plain compact due date", humanList)
		}
		if edited.DueOn == nil || *edited.DueOn != yesterday ||
			len(overdue) != 1 || overdue[0].ID != created.ID {
			t.Errorf("edited/overdue = %#v/%#v, want yesterday and created task", edited, overdue)
		}
		if len(openOverdue) != 0 || len(doneOverdue) != 0 {
			t.Errorf("resolved overdue lists = %#v/%#v, want empty", openOverdue, doneOverdue)
		}
		if cleared.DueOn != nil || len(dueAfterClear) != 0 {
			t.Errorf("cleared/due list = %#v/%#v, want null due and empty list", cleared, dueAfterClear)
		}

		if todayDeferred.DeferUntil == nil || *todayDeferred.DeferUntil != capturedDate ||
			tomorrowDeferred.DeferUntil == nil || *tomorrowDeferred.DeferUntil != tomorrow ||
			tomorrowDeferred.DueOn == nil || *tomorrowDeferred.DueOn != capturedDate ||
			weekDeferred.DeferUntil == nil || *weekDeferred.DeferUntil != weekLater ||
			dueToday.DueOn == nil || *dueToday.DueOn != capturedDate {
			t.Errorf("canonical deferrals = %#v/%#v/%#v/%#v, want today, tomorrow with due today, +1w, and due today", todayDeferred, tomorrowDeferred, weekDeferred, dueToday)
		}
		if len(available) != 4 || available[0].ID != created.ID ||
			available[1].ID != undated.ID || available[2].ID != todayDeferred.ID ||
			available[3].ID != dueToday.ID {
			t.Errorf("available = %#v, want deadline, undated, today-deferred, and due-today tasks", available)
		}
		if len(deferred) != 2 || deferred[0].ID != tomorrowDeferred.ID || deferred[1].ID != weekDeferred.ID {
			t.Errorf("deferred = %#v, want tomorrow and week-deferred tasks", deferred)
		}
		normalizedHumanDeferred := strings.Join(strings.Fields(humanDeferred.stdout), " ")
		if humanDeferred.exitCode != 0 || humanDeferred.stderr != "" ||
			!strings.Contains(normalizedHumanDeferred, "Due on "+capturedDate) ||
			!strings.Contains(normalizedHumanDeferred, "Defer until "+tomorrow) ||
			strings.Contains(humanDeferred.stdout, "\x1b[") {
			t.Errorf("human deferred show = %#v, want plain labeled due and defer dates with values", humanDeferred)
		}
		if humanAvailable.exitCode != 0 || humanAvailable.stderr != "" ||
			!strings.Contains(humanAvailable.stdout, "defer "+capturedDate) ||
			strings.Contains(humanAvailable.stdout, "\x1b[") {
			t.Errorf("human available = %#v, want plain compact defer date", humanAvailable)
		}
		if clearedDefer.DeferUntil != nil || clearedDefer.DueOn == nil || *clearedDefer.DueOn != capturedDate ||
			len(availableAfterClear) != 5 || availableAfterClear[3].ID != tomorrowDeferred.ID ||
			availableAfterClear[4].ID != dueToday.ID ||
			len(deferredAfterClear) != 1 || deferredAfterClear[0].ID != weekDeferred.ID {
			t.Errorf("cleared defer results = %#v/%#v/%#v, want immediate availability and preserved due date", clearedDefer, availableAfterClear, deferredAfterClear)
		}
		if len(openDeferred) != 0 || len(doneDeferred) != 1 || doneDeferred[0].ID != weekDeferred.ID {
			t.Errorf("resolved deferred lists = %#v/%#v, want default exclusion and done inclusion", openDeferred, doneDeferred)
		}
		if len(overdueBeforeAdd) != 0 || len(addedOverdueList) != 1 || addedOverdueList[0].ID != addedOverdue.ID ||
			len(addedOpenOverdue) != 0 || len(addedDoneOverdue) != 0 {
			t.Errorf("added overdue workflow = %#v/%#v/%#v/%#v, want one new overdue task then empty resolved lists", overdueBeforeAdd, addedOverdueList, addedOpenOverdue, addedDoneOverdue)
		}
		for index, current := range weekdayDeferred {
			if current.DeferUntil == nil || *current.DeferUntil != weekdayDates[index] {
				t.Errorf("weekday %s defer = %#v, want %s", weekdayCases[index].token, current.DeferUntil, weekdayDates[index])
			}
		}

		all := decodeTasks(t, runGSD(t, "list", "--status", "all", "--db", databasePath, "--json"))
		if len(all) != 14 {
			t.Errorf("all tasks after rejected adds = %#v, want fourteen accepted tasks", all)
		}
		return
	}

	t.Fatal("local date rolled over during every time workflow attempt")
}

func calendarDate(reference time.Time, days int) string {
	year, month, day := reference.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, reference.Location()).
		AddDate(0, 0, days).
		Format("2006-01-02")
}

func writeE2EConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}

func decodeTask(t *testing.T, result processResult) task.Task {
	t.Helper()
	return decodeJSON[task.Task](t, result, "task")
}

func decodeTasks(t *testing.T, result processResult) []task.Task {
	t.Helper()
	return decodeJSON[[]task.Task](t, result, "tasks")
}

func decodeJSON[T any](t *testing.T, result processResult, description string) T {
	t.Helper()
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("command result = %#v, want JSON success", result)
	}
	if !strings.HasSuffix(result.stdout, "\n") || strings.Count(result.stdout, "\n") != 1 {
		t.Fatalf("stdout = %q, want one newline-terminated JSON value", result.stdout)
	}

	var decoded T
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode %s: %v", description, err)
	}
	return decoded
}

func assertJSONError(t *testing.T, result processResult, wantCode apperr.Code) {
	t.Helper()
	if result.exitCode != 1 || result.stdout != "" {
		t.Fatalf("command result = %#v, want stderr-only exit 1", result)
	}
	if !strings.HasSuffix(result.stderr, "\n") || strings.Count(result.stderr, "\n") != 1 {
		t.Fatalf("stderr = %q, want one newline-terminated JSON value", result.stderr)
	}

	var envelope struct {
		Error struct {
			Code apperr.Code `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q", envelope.Error.Code, wantCode)
	}
}
