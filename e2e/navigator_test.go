package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNavigatorRootInTerminal(t *testing.T) {
	workflowDir, err := os.MkdirTemp(workDir, "navigator-")
	if err != nil {
		t.Fatalf("create navigator workflow directory: %v", err)
	}
	databasePath := filepath.Join(workflowDir, "navigator.db")
	seedNavigatorDatabase(t, databasePath)

	session := startTerminalSession(t, workflowDir, terminalSpec{
		name:       "navigator",
		dimensions: terminalDimensions{width: 80, height: 24},
		command:    []string{binaryPath, "--db", databasePath, "tui"},
	})
	labels := []string{"Inbox", "Available", "Logbook", "Boards", "Areas"}
	activePane := session.waitForPane(func(pane string) bool {
		return containsInOrder(pane, labels...)
	})
	if session.exited() {
		t.Fatalf("navigator exited before active pane capture; pane = %q", activePane)
	}
	if !containsInOrder(activePane, labels...) {
		t.Fatalf("navigator pane = %q, want root labels in order %v", activePane, labels)
	}

	session.sendKeys("q")
	if exitCode := session.waitForExit(); exitCode != 0 {
		t.Errorf("navigator exit code = %d, want 0; active pane = %q", exitCode, activePane)
	}
}

func seedNavigatorDatabase(t *testing.T, databasePath string) {
	t.Helper()
	commands := [][]string{
		{"add", "Call plumber"},
		{"boards", "add", "Software", "--stage", "Research", "--stage", "Doing", "--stage", "Review"},
		{"areas", "add", "Home"},
		{"areas", "add", "Work"},
	}
	for _, command := range commands {
		arguments := append(command, "--db", databasePath, "--json")
		result := runGSD(t, arguments...)
		if result.exitCode != 0 || result.stderr != "" {
			t.Fatalf("seed navigator database with %v: %#v", command, result)
		}
	}
}

func containsInOrder(content string, values ...string) bool {
	for _, value := range values {
		index := strings.Index(content, value)
		if index < 0 {
			return false
		}
		content = content[index+len(value):]
	}
	return true
}
