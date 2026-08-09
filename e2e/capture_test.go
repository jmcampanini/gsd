package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/task"
)

type terminalCaptureResult struct {
	exitCode int
	pane     string
}

func TestCaptureTerminalWorkflow(t *testing.T) {
	workflowDir, err := os.MkdirTemp(workDir, "terminal-capture-")
	if err != nil {
		t.Fatalf("create terminal capture directory: %v", err)
	}

	databasePath := filepath.Join(workflowDir, "explicit.db")
	title := "Captured in a terminal"
	added := runTerminalCapture(t, workflowDir, nil, []string{"--db", databasePath, "capture"}, title, "Enter")
	assertTerminalCaptureSuccess(t, added)

	tasks := decodeTasks(t, runGSD(t, "inbox", "--db", databasePath, "--json"))
	if len(tasks) != 1 {
		t.Fatalf("explicit database inbox = %#v, want exactly one task", tasks)
	}
	assertTitleOnlyTask(t, tasks[0], title)

	cancelled := runTerminalCapture(
		t,
		workflowDir,
		nil,
		[]string{"--db", databasePath, "capture"},
		"Discarded in a terminal",
		"Escape",
	)
	assertTerminalCaptureSuccess(t, cancelled)
	afterCancel := decodeTasks(t, runGSD(t, "inbox", "--db", databasePath, "--json"))
	if len(afterCancel) != 1 || afterCancel[0].ID != tasks[0].ID {
		t.Errorf("inbox after Escape = %#v, want no new row", afterCancel)
	}

	environmentDatabasePath := filepath.Join(workflowDir, "environment.db")
	environmentTitle := "Captured from GSD_DB"
	fromEnvironment := runTerminalCapture(
		t,
		workflowDir,
		map[string]string{"GSD_DB": environmentDatabasePath},
		[]string{"capture"},
		environmentTitle,
		"Enter",
	)
	assertTerminalCaptureSuccess(t, fromEnvironment)
	environmentTasks := decodeTasks(t, runGSD(t, "inbox", "--db", environmentDatabasePath, "--json"))
	if len(environmentTasks) != 1 {
		t.Fatalf("environment database inbox = %#v, want exactly one task", environmentTasks)
	}
	assertTitleOnlyTask(t, environmentTasks[0], environmentTitle)
}

func TestCaptureRejectsNoninteractiveInvocationWithoutWriting(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		diagnostic string
		arguments  func(string) []string
	}{
		{
			name:       "JSON",
			diagnostic: "--json",
			arguments: func(databasePath string) []string {
				return []string{"--db", databasePath, "capture", "--json"}
			},
		},
		{
			name:       "piped input",
			input:      "not terminal input\n",
			diagnostic: "gsd add",
			arguments: func(databasePath string) []string {
				return []string{"--db", databasePath, "capture"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			databasePath := filepath.Join(workDir, "capture-"+strings.ReplaceAll(test.name, " ", "-")+".db")
			result := runGSDWithInput(t, test.input, test.arguments(databasePath)...)
			if result.exitCode != 2 || result.stdout != "" {
				t.Fatalf("capture result = %#v, want stderr-only exit 2", result)
			}
			if !strings.Contains(result.stderr, test.diagnostic) {
				t.Errorf("stderr = %q, want actionable diagnostic containing %q", result.stderr, test.diagnostic)
			}
			if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("database stat error = %v, want no database written", err)
			}
		})
	}
}

func runTerminalCapture(
	t *testing.T,
	workflowDir string,
	environment map[string]string,
	arguments []string,
	title string,
	key string,
) (result terminalCaptureResult) {
	t.Helper()

	command := append([]string{binaryPath}, arguments...)
	session := startTerminalSession(t, workflowDir, terminalSpec{
		name:        "capture",
		dimensions:  terminalDimensions{width: 62, height: 2},
		environment: environment,
		command:     command,
	})
	defer session.close()

	session.waitForPaneContaining("enter add")
	if title != "" {
		session.sendLiteral(title)
	}
	session.sendKeys(key)

	result.exitCode = session.waitForExit()
	result.pane = session.capturePane()
	return result
}

func assertTerminalCaptureSuccess(t *testing.T, result terminalCaptureResult) {
	t.Helper()
	if result.exitCode != 0 {
		t.Errorf("terminal capture exit code = %d, want 0", result.exitCode)
	}
	if strings.TrimSpace(result.pane) != "" {
		t.Errorf("terminal capture pane = %q, want no residual output", result.pane)
	}
}

func assertTitleOnlyTask(t *testing.T, captured task.Task, title string) {
	t.Helper()
	if captured.Title != title || captured.Note != "" || captured.ProjectID != nil || captured.AreaID != nil ||
		captured.DeferUntil != nil || captured.DueOn != nil || captured.DoneAt != nil || captured.CancelledAt != nil ||
		captured.Status != "open" || len(captured.Tags) != 0 {
		t.Errorf("captured task = %#v, want title-only open task %q", captured, title)
	}
}
