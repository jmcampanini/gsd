package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	binaryPath      string
	expectedVersion string
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

	workDir, err := os.MkdirTemp(sandbox, "e2e-")
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := exec.Command(binaryPath, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

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
