package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/task"
)

const capturePaneTarget = "capture:0.0"

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

	socketPath := filepath.Base(workflowDir) + ".sock"
	statusPath := filepath.Join(workflowDir, "capture.status")
	if err := os.Remove(statusPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove stale capture status: %v", err)
	}

	commandArguments := append([]string{binaryPath}, arguments...)
	quotedArguments := make([]string, len(commandArguments))
	for index, argument := range commandArguments {
		quotedArguments[index] = shellQuote(argument)
	}
	wrapperScript := strings.Join(quotedArguments, " ") +
		"; capture_status=$?; printf '%s\\n' \"$capture_status\" > " + shellQuote(statusPath) +
		"; while :; do sleep 3600; done"
	wrapper := "exec /bin/sh -c " + shellQuote(wrapperScript)

	start := tmuxCommand(socketPath, environment,
		"new-session", "-d", "-s", "capture", "-x", "62", "-y", "2", wrapper,
	)
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start private tmux server: %v: %s", err, output)
	}
	defer func() {
		kill := tmuxCommand(socketPath, environment, "kill-server")
		if output, err := kill.CombinedOutput(); err != nil {
			t.Errorf("kill private tmux server: %v: %s", err, output)
		}
		_ = os.Remove(filepath.Join(workDir, socketPath))
	}()

	dimensions := tmuxCommand(socketPath, environment,
		"display-message", "-p", "-t", capturePaneTarget, "#{pane_width}x#{pane_height}",
	)
	output, err := dimensions.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "62x2" {
		t.Fatalf("capture pane dimensions = %q, error = %v; want 62x2", output, err)
	}

	waitForPane(t, socketPath, environment, "enter add")
	if title != "" {
		sendLiteral := tmuxCommand(socketPath, environment, "send-keys", "-t", capturePaneTarget, "-l", title)
		if output, err := sendLiteral.CombinedOutput(); err != nil {
			t.Fatalf("send literal capture title: %v: %s", err, output)
		}
	}
	sendKey := tmuxCommand(socketPath, environment, "send-keys", "-t", capturePaneTarget, key)
	if output, err := sendKey.CombinedOutput(); err != nil {
		t.Fatalf("send named %s key: %v: %s", key, err, output)
	}

	result.exitCode = waitForCaptureStatus(t, statusPath)
	result.pane = capturePane(t, socketPath, environment)
	return result
}

func tmuxCommand(socketPath string, environment map[string]string, arguments ...string) *exec.Cmd {
	commandArguments := append([]string{"-S", socketPath, "-f", "/dev/null"}, arguments...)
	command := exec.Command(tmuxPath, commandArguments...)
	command.Dir = workDir
	command.Env = filteredEnvironment(environment)
	return command
}

func waitForPane(t *testing.T, socketPath string, environment map[string]string, content string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastPane string
	for time.Now().Before(deadline) {
		lastPane = capturePane(t, socketPath, environment)
		if strings.Contains(lastPane, content) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("tmux capture did not become ready; last pane = %q", lastPane)
}

func waitForCaptureStatus(t *testing.T, statusPath string) int {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	var lastError error
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(statusPath)
		if err == nil {
			exitCode, parseErr := strconv.Atoi(strings.TrimSpace(string(content)))
			if parseErr == nil {
				return exitCode
			}
			lastError = parseErr
		} else {
			lastError = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("capture process did not write an exit status: %v", lastError)
	return -1
}

func capturePane(t *testing.T, socketPath string, environment map[string]string) string {
	t.Helper()
	capture := tmuxCommand(socketPath, environment, "capture-pane", "-p", "-t", capturePaneTarget)
	output, err := capture.CombinedOutput()
	if err != nil {
		t.Fatalf("capture tmux pane: %v: %s", err, output)
	}
	return string(output)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
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
