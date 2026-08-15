package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNavigatorWorkflowInTerminal(t *testing.T) {
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

	session.sendKeys("j", "Enter")
	availablePane := session.waitForPane(func(pane string) bool {
		return strings.Contains(pane, "Call plumber") && strings.Contains(pane, "Plan meal")
	})

	session.sendKeys("/")
	session.waitForPane(func(pane string) bool {
		return pane != availablePane && strings.Contains(pane, "Call plumber") && strings.Contains(pane, "Plan meal")
	})
	session.sendLiteral("plmb")
	filteredAvailablePane := session.waitForPane(func(pane string) bool {
		return strings.Contains(pane, "Call plumber") && !strings.Contains(pane, "Plan meal")
	})
	if !strings.Contains(filteredAvailablePane, "Call plumber") || strings.Contains(filteredAvailablePane, "Plan meal") {
		t.Fatalf("filtered Available pane = %q, want plumber present and nonmatch absent", filteredAvailablePane)
	}

	session.sendKeys("Escape")
	restoredAvailablePane := session.waitForPane(func(pane string) bool {
		return strings.Contains(pane, "Call plumber") && strings.Contains(pane, "Plan meal")
	})
	if !strings.Contains(restoredAvailablePane, "Call plumber") || !strings.Contains(restoredAvailablePane, "Plan meal") {
		t.Fatalf("restored Available pane = %q, want both tasks present", restoredAvailablePane)
	}

	session.sendKeys("Escape")
	rootPane := session.waitForPane(func(pane string) bool {
		return containsInOrder(pane, labels...) && !strings.Contains(pane, "Call plumber")
	})
	if !containsInOrder(rootPane, labels...) {
		t.Fatalf("navigator pane = %q, want restored root labels in order %v", rootPane, labels)
	}

	session.sendKeys("j", "j", "j", "Enter")
	session.waitForPane(func(pane string) bool {
		return containsInOrder(pane, "Home", "Work", "(no area)")
	})
	session.sendKeys("Enter")
	homePane := session.waitForPane(func(pane string) bool {
		return strings.Contains(pane, "Kitchen reno") && strings.Contains(pane, "Refresh entry")
	})

	session.sendKeys("/")
	session.waitForPane(func(pane string) bool {
		return pane != homePane && strings.Contains(pane, "Kitchen reno") && strings.Contains(pane, "Refresh entry")
	})
	session.sendLiteral("reno")
	filteredHomePane := session.waitForPane(func(pane string) bool {
		return strings.Contains(pane, "Kitchen reno") && !strings.Contains(pane, "Refresh entry")
	})
	if !strings.Contains(filteredHomePane, "Kitchen reno") || strings.Contains(filteredHomePane, "Refresh entry") {
		t.Fatalf("filtered Home pane = %q, want Kitchen reno present and nonmatch absent", filteredHomePane)
	}

	session.sendKeys("/")
	session.waitForPane(func(pane string) bool {
		return pane != filteredHomePane && strings.Contains(pane, "Kitchen reno") && !strings.Contains(pane, "Refresh entry")
	})
	session.sendKeys("q")
	if exitCode := session.waitForExit(); exitCode != 0 {
		t.Errorf("navigator exit code = %d, want 0; last pane = %q", exitCode, filteredHomePane)
	}
}

func seedNavigatorDatabase(t *testing.T, databasePath string) {
	t.Helper()
	commands := [][]string{
		{"add", "Call plumber"},
		{"add", "Plan meal"},
		{"boards", "add", "Software", "--stage", "Research", "--stage", "Doing", "--stage", "Review"},
		{"areas", "add", "Home"},
		{"areas", "add", "Work"},
		{"projects", "add", "Kitchen reno", "--area", "1"},
		{"projects", "add", "Refresh entry", "--area", "1"},
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
