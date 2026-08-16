package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const terminalPollTimeout = 10 * time.Second

type terminalDimensions struct {
	width  int
	height int
}

type terminalSpec struct {
	name        string
	dimensions  terminalDimensions
	environment map[string]string
	command     []string
}

type terminalSession struct {
	t           *testing.T
	socketPath  string
	statusPath  string
	target      string
	environment map[string]string
	closed      bool
}

func startTerminalSession(t *testing.T, workflowDir string, spec terminalSpec) *terminalSession {
	t.Helper()

	if len(spec.command) == 0 {
		t.Fatal("start terminal session: command is required")
	}

	// A relative socket keeps long checkout paths below the UNIX-socket path limit.
	socketPath := filepath.Base(workflowDir) + "-" + spec.name + ".sock"
	statusPath := filepath.Join(workflowDir, spec.name+".status")
	if err := os.Remove(statusPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove stale terminal status: %v", err)
	}

	quotedCommand := make([]string, len(spec.command))
	for index, argument := range spec.command {
		quotedCommand[index] = shellQuote(argument)
	}
	wrapperScript := strings.Join(quotedCommand, " ") +
		"; terminal_status=$?; printf '%s\\n' \"$terminal_status\" > " + shellQuote(statusPath) +
		"; while :; do sleep 3600; done"
	wrapper := "exec /bin/sh -c " + shellQuote(wrapperScript)

	session := &terminalSession{
		t:           t,
		socketPath:  socketPath,
		statusPath:  statusPath,
		target:      spec.name + ":0.0",
		environment: spec.environment,
	}
	start := session.tmuxCommand(
		"new-session",
		"-d",
		"-s", spec.name,
		"-x", strconv.Itoa(spec.dimensions.width),
		"-y", strconv.Itoa(spec.dimensions.height),
		wrapper,
	)
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start private tmux server: %v: %s", err, output)
	}
	t.Cleanup(session.close)

	wantDimensions := fmt.Sprintf("%dx%d", spec.dimensions.width, spec.dimensions.height)
	dimensions := session.tmuxCommand(
		"display-message", "-p", "-t", session.target, "#{pane_width}x#{pane_height}",
	)
	output, err := dimensions.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != wantDimensions {
		t.Fatalf("terminal pane dimensions = %q, error = %v; want %s", output, err, wantDimensions)
	}

	return session
}

func (s *terminalSession) close() {
	if s.closed {
		return
	}
	s.closed = true
	output, err := s.tmuxCommand("kill-server").CombinedOutput()
	if err != nil && !strings.Contains(string(output), "no server running") {
		s.t.Errorf("kill private tmux server: %v: %s", err, output)
	}
	_ = os.Remove(filepath.Join(workDir, s.socketPath))
}

func (s *terminalSession) sendLiteral(value string) {
	s.t.Helper()
	command := s.tmuxCommand("send-keys", "-t", s.target, "-l", value)
	if output, err := command.CombinedOutput(); err != nil {
		s.t.Fatalf("send literal terminal input: %v: %s", err, output)
	}
}

func (s *terminalSession) sendKeys(keys ...string) {
	s.t.Helper()
	arguments := append([]string{"send-keys", "-t", s.target}, keys...)
	command := s.tmuxCommand(arguments...)
	if output, err := command.CombinedOutput(); err != nil {
		s.t.Fatalf("send terminal keys %v: %v: %s", keys, err, output)
	}
}

func (s *terminalSession) waitForPane(predicate func(string) bool) string {
	s.t.Helper()

	deadline := time.Now().Add(terminalPollTimeout)
	var lastPane string
	for time.Now().Before(deadline) {
		lastPane = s.capturePane()
		if predicate(lastPane) {
			return lastPane
		}
		if s.exited() {
			s.t.Fatalf("terminal process exited with status %d before pane became ready; last pane = %q", s.waitForExit(), lastPane)
		}
		time.Sleep(25 * time.Millisecond)
	}
	s.t.Fatalf("terminal pane did not become ready; last pane = %q", lastPane)
	return ""
}

func (s *terminalSession) waitForPaneContaining(content string) string {
	s.t.Helper()
	return s.waitForPane(func(pane string) bool {
		return strings.Contains(pane, content)
	})
}

func (s *terminalSession) capturePane() string {
	s.t.Helper()
	capture := s.tmuxCommand("capture-pane", "-p", "-t", s.target)
	output, err := capture.CombinedOutput()
	if err != nil {
		s.t.Fatalf("capture terminal pane: %v: %s", err, output)
	}
	return string(output)
}

func (s *terminalSession) exited() bool {
	s.t.Helper()
	_, err := os.Stat(s.statusPath)
	if err == nil {
		return true
	}
	if !errors.Is(err, os.ErrNotExist) {
		s.t.Fatalf("stat terminal status: %v", err)
	}
	return false
}

func (s *terminalSession) waitForExit() int {
	s.t.Helper()

	deadline := time.Now().Add(terminalPollTimeout)
	var lastError error
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(s.statusPath)
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
	s.t.Fatalf("terminal process did not write an exit status: %v", lastError)
	return -1
}

func (s *terminalSession) tmuxCommand(arguments ...string) *exec.Cmd {
	commandArguments := append([]string{"-S", s.socketPath, "-f", "/dev/null"}, arguments...)
	command := exec.Command(tmuxPath, commandArguments...)
	command.Dir = workDir
	command.Env = filteredEnvironment(s.environment)
	return command
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
