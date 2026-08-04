package config_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	configloader "github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/spf13/pflag"
)

func TestLoadAppliesDatabasePathPrecedenceAndProvenance(t *testing.T) {
	type paths struct {
		config string
		data   string
	}
	type expectation struct {
		path   string
		source string
	}
	tests := []struct {
		name        string
		fileValue   string
		environment string
		flag        string
		want        func(paths) expectation
	}{
		{
			name: "default",
			want: func(current paths) expectation {
				return expectation{
					path:   filepath.Join(current.data, "gsd", "gsd.db"),
					source: configloader.SourceDefault,
				}
			},
		},
		{
			name:      "file",
			fileValue: "file.db",
			want: func(current paths) expectation {
				return expectation{
					path:   filepath.Join(filepath.Dir(current.config), "file.db"),
					source: current.config,
				}
			},
		},
		{
			name:        "environment",
			fileValue:   "file.db",
			environment: "environment.db",
			want: func(paths) expectation {
				return expectation{path: "environment.db", source: configloader.SourceEnv}
			},
		},
		{
			name:        "flag",
			fileValue:   "file.db",
			environment: "environment.db",
			flag:        "flag.db",
			want: func(paths) expectation {
				return expectation{path: "flag.db", source: pflagloader.SourcePFlag}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			dataHome := filepath.Join(dir, "data")
			configHome := filepath.Join(dir, "config")
			t.Setenv("HOME", filepath.Join(dir, "home"))
			t.Setenv("XDG_CONFIG_HOME", configHome)
			t.Setenv("XDG_DATA_HOME", dataHome)
			t.Setenv("GSD_DB", test.environment)

			configPath := filepath.Join(configHome, "gsd", "config.toml")
			if test.fileValue != "" {
				writeConfig(t, configPath, fmt.Sprintf("db_path = %q\n", test.fileValue))
			}
			flags := newFlags(t, flagArgs(test.flag)...)

			loaded, report, err := config.Load("", false, flags)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			want := test.want(paths{config: configPath, data: dataHome})
			if loaded.DBPath != want.path {
				t.Errorf("Load() DBPath = %q, want %q", loaded.DBPath, want.path)
			}
			if report.Updates["dbpath"] != want.source {
				t.Errorf("Load() source = %q, want %q", report.Updates["dbpath"], want.source)
			}
		})
	}
}

func TestLoadUsesExplicitFileInsteadOfDiscoveryBeforeHigherPrioritySources(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, "config")
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GSD_DB", "")

	discovered := filepath.Join(configHome, "gsd", "config.toml")
	explicit := filepath.Join(dir, "explicit", "chosen.toml")
	writeConfig(t, discovered, "db_path = 'discovered.db'\n")
	writeConfig(t, explicit, "db_path = 'explicit.db'\n")

	loaded, report, err := config.Load(explicit, true, newFlags(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := filepath.Join(filepath.Dir(explicit), "explicit.db")
	if loaded.DBPath != want {
		t.Errorf("Load() DBPath = %q, want %q", loaded.DBPath, want)
	}
	if len(report.LoadedFiles) != 1 || report.LoadedFiles[0] != explicit {
		t.Errorf("Load() files = %#v, want only %q", report.LoadedFiles, explicit)
	}
}

func TestLoadKeepsRelativeEnvironmentAndFlagPathsWorkingDirectoryRelative(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GSD_DB", "environment.db")

	environment, _, err := config.Load("", false, newFlags(t))
	if err != nil {
		t.Fatalf("Load(environment) error = %v", err)
	}
	if environment.DBPath != "environment.db" {
		t.Errorf("environment DBPath = %q, want unchanged relative path", environment.DBPath)
	}

	flag, _, err := config.Load("", false, newFlags(t, "--db", "flag.db"))
	if err != nil {
		t.Fatalf("Load(flag) error = %v", err)
	}
	if flag.DBPath != "flag.db" {
		t.Errorf("flag DBPath = %q, want unchanged relative path", flag.DBPath)
	}
}

func TestLoadIgnoresEmptyLegacyOverrides(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, "config")
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GSD_DB", "")

	configPath := filepath.Join(configHome, "gsd", "config.toml")
	writeConfig(t, configPath, "db_path = 'file.db'\n")
	loaded, report, err := config.Load("", false, newFlags(t, "--db="))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := filepath.Join(filepath.Dir(configPath), "file.db")
	if loaded.DBPath != want || report.Updates["dbpath"] != configPath {
		t.Errorf("Load() = %#v, source %q; want file value %q and source", loaded, report.Updates["dbpath"], want)
	}
}

func TestLoadDefaultsToHomeDataDirectoryWithoutXDGDataHome(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GSD_DB", "")

	loaded, report, err := config.Load("", false, newFlags(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := filepath.Join(home, ".local", "share", "gsd", "gsd.db")
	if loaded.DBPath != want || report.Updates["dbpath"] != configloader.SourceDefault {
		t.Errorf("Load() = %#v, source %q; want default %q", loaded, report.Updates["dbpath"], want)
	}
}

func TestLoadRejectsEmptyFilePathBeforeHigherPriorityOverrides(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		flagArgs    []string
	}{
		{name: "environment", environment: "environment.db"},
		{name: "flag", flagArgs: []string{"--db", "flag.db"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			writeConfig(t, path, "db_path = ''\n")
			t.Setenv("HOME", filepath.Join(dir, "home"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			t.Setenv("GSD_DB", test.environment)

			_, _, err := config.Load(path, true, newFlags(t, test.flagArgs...))
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.InvalidArgument {
				t.Errorf("Load() error = %v, want invalid_argument", err)
			}
		})
	}
}

func TestLoadFailsWhenDefaultHomeIsUnavailableWithoutOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GSD_DB", "")

	_, _, err := config.Load("", false, newFlags(t))
	if err == nil {
		t.Fatal("Load() error = nil, want unavailable default home error")
	}
}

func TestLoadDoesNotRequireDefaultHomeWhenLegacyOverrideExists(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		flagArgs    []string
		want        string
		wantSource  string
	}{
		{name: "environment", environment: "environment.db", want: "environment.db", wantSource: configloader.SourceEnv},
		{name: "flag", flagArgs: []string{"--db", "flag.db"}, want: "flag.db", wantSource: pflagloader.SourcePFlag},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", "")
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("XDG_DATA_HOME", "")
			t.Setenv("GSD_DB", test.environment)

			loaded, report, err := config.Load("", false, newFlags(t, test.flagArgs...))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if loaded.DBPath != test.want || report.Updates["dbpath"] != test.wantSource {
				t.Errorf("Load() = %#v, source %q; want %q from %q", loaded, report.Updates["dbpath"], test.want, test.wantSource)
			}
		})
	}
}

func TestLoadRejectsConfigFailuresAsInvalidArguments(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string) string
	}{
		{
			name: "missing explicit file",
			setup: func(_ *testing.T, dir string) string {
				return filepath.Join(dir, "missing.toml")
			},
		},
		{
			name: "unreadable explicit location",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				blocked := filepath.Join(dir, "blocked")
				if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
					t.Fatalf("write blocked parent: %v", err)
				}
				return filepath.Join(blocked, "config.toml")
			},
		},
		{
			name: "invalid TOML",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "invalid.toml")
				writeConfig(t, path, "db_path = [\n")
				return path
			},
		},
		{
			name: "unknown key",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "unknown.toml")
				writeConfig(t, path, "color = 'always'\n")
				return path
			},
		},
		{
			name: "empty database path",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "empty.toml")
				writeConfig(t, path, "db_path = ''\n")
				return path
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", filepath.Join(dir, "home"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			t.Setenv("GSD_DB", "")

			_, _, err := config.Load(test.setup(t, dir), true, newFlags(t))
			if err == nil {
				t.Fatal("Load() error = nil, want invalid_argument")
			}
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.InvalidArgument {
				t.Errorf("Load() error = %v, want invalid_argument", err)
			}
		})
	}
}

func TestLoadTreatsOnlyAbsentDiscoveredConfigAsOptional(t *testing.T) {
	tests := []struct {
		name    string
		content string
		makeDir bool
		env     string
		wantErr bool
	}{
		{name: "missing"},
		{name: "directory at discovered path", makeDir: true},
		{name: "invalid existing file", content: "db_path = [\n", wantErr: true},
		{
			name:    "invalid existing file with valid env override",
			content: "db_path = [\n",
			env:     "override.db",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			configHome := filepath.Join(dir, "config")
			t.Setenv("HOME", filepath.Join(dir, "home"))
			t.Setenv("XDG_CONFIG_HOME", configHome)
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			t.Setenv("GSD_DB", test.env)

			path := filepath.Join(configHome, "gsd", "config.toml")
			switch {
			case test.makeDir:
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatalf("create discovered directory: %v", err)
				}
			case test.content != "":
				writeConfig(t, path, test.content)
			}

			_, _, err := config.Load("", false, newFlags(t))
			if test.wantErr {
				code, ok := apperr.CodeOf(err)
				if !ok || code != apperr.InvalidArgument {
					t.Errorf("Load() error = %v, want invalid_argument", err)
				}
			} else if err != nil {
				t.Errorf("Load() error = %v, want optional discovery success", err)
			}
		})
	}
}

func newFlags(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if err := config.RegisterFlags(flags); err != nil {
		t.Fatalf("RegisterFlags() error = %v", err)
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return flags
}

func flagArgs(value string) []string {
	if value == "" {
		return nil
	}
	return []string{"--db", value}
}

func writeConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
