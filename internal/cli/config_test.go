package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/spf13/pflag"
)

func TestWriteConfigReportRendersLoadedValueThenEffectiveBlock(t *testing.T) {
	t.Parallel()

	loaded := config.Loaded{
		Config:    config.Config{DBPath: "relative/gsd.db"},
		Effective: config.Effective{DBPath: "/work/relative/gsd.db"},
	}

	var plain bytes.Buffer
	if err := writeConfigReport(&plain, loaded, false); err != nil {
		t.Fatalf("writeConfigReport(plain) error = %v", err)
	}

	want := "db_path = \"relative/gsd.db\"\n" +
		"\n# Effective\n" +
		"# effective_db_path = \"/work/relative/gsd.db\"\n"
	if plain.String() != want {
		t.Errorf("plain report = %q, want %q", plain.String(), want)
	}
}

func TestWriteConfigReportCommentsProvenanceWithActionableSources(t *testing.T) {
	t.Parallel()

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

			loaded := config.Loaded{
				Config:    config.Config{DBPath: "relative/gsd.db"},
				Effective: config.Effective{DBPath: "/work/relative/gsd.db"},
				Report:    configloader.LoadReport{Updates: configloader.Updates{config.DBPathKey: test.source}},
			}

			var output bytes.Buffer
			if err := writeConfigReport(&output, loaded, true); err != nil {
				t.Fatalf("writeConfigReport(provenance) error = %v", err)
			}

			want := fmt.Sprintf("db_path = \"relative/gsd.db\" # %s\n", test.want) +
				"\n# Effective\n" +
				"# effective_db_path = \"/work/relative/gsd.db\"\n"
			if output.String() != want {
				t.Errorf("provenance report = %q, want %q", output.String(), want)
			}
		})
	}
}

func TestConfigCommandLoadsConfigurationWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	loads := 0
	loader := func(path string, explicit bool, flags *pflag.FlagSet) (config.Loaded, error) {
		loads++
		databasePath, _ := flags.GetString("db")
		if path != "chosen.toml" || !explicit || databasePath != "chosen.db" {
			t.Errorf("load inputs = (%q, %t, %q), want chosen paths", path, explicit, databasePath)
		}
		return config.Loaded{
			Config:    config.Config{DBPath: databasePath},
			Effective: config.Effective{DBPath: "/work/chosen.db"},
			Report: configloader.LoadReport{
				Updates: configloader.Updates{config.DBPathKey: pflagloader.SourcePFlag},
			},
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
		"--color=always",
	)
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	want := "db_path = \"chosen.db\" # flag: --db\n" +
		"\n# Effective\n" +
		"# effective_db_path = \"/work/chosen.db\"\n"
	if result.stdout != want {
		t.Errorf("stdout = %q, want effective config %q with flag provenance", result.stdout, want)
	}
	if loads != 1 || result.opens != 0 || result.closes != 0 {
		t.Errorf("lifecycle = loads %d, opens %d, closes %d; want 1, 0, 0", loads, result.opens, result.closes)
	}
}

func TestConfigCommandRejectsJSONBeforeLoadingConfiguration(t *testing.T) {
	t.Parallel()

	loads := 0
	loader := func(string, bool, *pflag.FlagSet) (config.Loaded, error) {
		loads++
		return config.Loaded{}, nil
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

	loader := func(string, bool, *pflag.FlagSet) (config.Loaded, error) {
		return config.Loaded{}, apperr.New(
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
