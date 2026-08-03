package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/spf13/pflag"
)

func TestWriteConfigReportRendersRedirectableTOMLAndActionableProvenance(t *testing.T) {
	t.Parallel()

	loaded := config.Config{DBPath: "relative/gsd.db"}
	effectivePath, err := filepath.Abs(loaded.DBPath)
	if err != nil {
		t.Fatalf("resolve expected effective path: %v", err)
	}
	var plain bytes.Buffer
	if err := writeConfigReport(&plain, loaded, configloader.LoadReport{}, false); err != nil {
		t.Fatalf("writeConfigReport(plain) error = %v", err)
	}
	wantPlain := fmt.Sprintf("db_path = %q\n", effectivePath)
	if plain.String() != wantPlain {
		t.Errorf("plain report = %q, want redirectable TOML %q", plain.String(), wantPlain)
	}

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "default", source: configloader.SourceDefault, want: "default"},
		{name: "file", source: "/config/home/gsd/config.toml\nforged", want: `file: /config/home/gsd/config.toml\nforged`},
		{name: "environment", source: configloader.SourceEnv, want: "env: GSD_DB"},
		{name: "flag", source: pflagloader.SourcePFlag, want: "flag: --db"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			report := configloader.LoadReport{Updates: configloader.Updates{dbPathReportKey: test.source}}
			if err := writeConfigReport(&output, loaded, report, true); err != nil {
				t.Fatalf("writeConfigReport(provenance) error = %v", err)
			}
			want := fmt.Sprintf("db_path = %q # %s\n", effectivePath, test.want)
			if output.String() != want {
				t.Errorf("provenance report = %q, want %q", output.String(), want)
			}
		})
	}
}

func TestConfigCommandLoadsConfigurationWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	loads := 0
	loader := func(
		path string,
		explicit bool,
		flags *pflag.FlagSet,
	) (config.Config, configloader.LoadReport, error) {
		loads++
		databasePath, _ := flags.GetString("db")
		if path != "chosen.toml" || !explicit || databasePath != "chosen.db" {
			t.Errorf("load inputs = (%q, %t, %q), want chosen paths", path, explicit, databasePath)
		}
		return config.Config{DBPath: databasePath}, configloader.LoadReport{
			Updates: configloader.Updates{dbPathReportKey: pflagloader.SourcePFlag},
		}, nil
	}

	result := runConfigurationCommand(
		t,
		loader,
		"config",
		"--config",
		"chosen.toml",
		"--db",
		"chosen.db",
		"--provenance",
	)
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	chosenPath, err := filepath.Abs("chosen.db")
	if err != nil {
		t.Fatalf("resolve expected chosen path: %v", err)
	}
	want := fmt.Sprintf("db_path = %q # flag: --db\n", chosenPath)
	if result.stdout != want {
		t.Errorf("stdout = %q, want effective config %q with flag provenance", result.stdout, want)
	}
	if loads != 1 || result.opens != 0 || result.closes != 0 {
		t.Errorf("lifecycle = loads %d, opens %d, closes %d; want 1, 0, 0", loads, result.opens, result.closes)
	}
}

func TestConfigCommandHelpDoesNotLoadConfiguration(t *testing.T) {
	t.Parallel()

	loads := 0
	loader := func(
		string,
		bool,
		*pflag.FlagSet,
	) (config.Config, configloader.LoadReport, error) {
		loads++
		return config.Config{}, configloader.LoadReport{}, nil
	}
	result := runConfigurationCommand(
		t,
		loader,
		"config",
		"--config",
		"/nonexistent.toml",
		"--help",
	)
	if result.exitCode != 0 || result.stderr != "" || loads != 0 || result.opens != 0 {
		t.Fatalf("result = %#v, loads = %d; want help without loading runtime dependencies", result, loads)
	}
	if !strings.Contains(result.stdout, "Usage:\n  gsd config [flags]") {
		t.Errorf("stdout = %q, want config help", result.stdout)
	}
}

func TestConfigCommandRejectsJSONBeforeLoadingConfiguration(t *testing.T) {
	t.Parallel()

	loads := 0
	loader := func(
		string,
		bool,
		*pflag.FlagSet,
	) (config.Config, configloader.LoadReport, error) {
		loads++
		return config.Config{}, configloader.LoadReport{}, nil
	}
	result := runConfigurationCommand(t, loader, "config", "--json")
	if result.exitCode != 2 || result.stdout != "" || loads != 0 || result.opens != 0 {
		t.Fatalf("result = %#v, loads = %d; want stderr-only usage error without loading", result, loads)
	}
	if !strings.Contains(result.stderr, "--json is not supported by gsd config") || strings.HasPrefix(result.stderr, "{") {
		t.Errorf("stderr = %q, want human-readable unsupported-mode diagnostic", result.stderr)
	}
}

func TestConfigCommandMapsLoadFailuresWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	loader := func(
		string,
		bool,
		*pflag.FlagSet,
	) (config.Config, configloader.LoadReport, error) {
		return config.Config{}, configloader.LoadReport{}, apperr.New(
			apperr.InvalidArgument,
			"invalid configuration: missing file",
			nil,
		)
	}
	result := runConfigurationCommand(t, loader, "config", "--config", "missing.toml")
	if result.exitCode != 1 || result.stdout != "" || result.opens != 0 {
		t.Fatalf("result = %#v, want stderr-only application error without database open", result)
	}
	if result.stderr != "Error: invalid configuration: missing file\n" {
		t.Errorf("stderr = %q, want config diagnostic", result.stderr)
	}
}

func runConfigurationCommand(
	t *testing.T,
	loader configurationLoader,
	args ...string,
) commandResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := commandResult{}
	factory := func(
		context.Context,
		string,
		bool,
		*pflag.FlagSet,
	) (applications, io.Closer, error) {
		result.opens++
		return applications{}, closeRecorder{close: func() { result.closes++ }}, nil
	}
	root := newRootCommandWithDependencies(factory, loader, time.Local)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	result.exitCode = execute(root, args)
	result.stdout = stdout.String()
	result.stderr = stderr.String()
	return result
}
