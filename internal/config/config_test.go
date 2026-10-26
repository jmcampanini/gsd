package config_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		value     string
		effective string
		source    string
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
				path := filepath.Join(current.data, "gsd", "gsd.db")
				return expectation{value: path, effective: path, source: configloader.SourceDefault}
			},
		},
		{
			name:      "file",
			fileValue: "file.db",
			want: func(current paths) expectation {
				return expectation{
					value:     "file.db",
					effective: filepath.Join(filepath.Dir(current.config), "file.db"),
					source:    current.config,
				}
			},
		},
		{
			name:        "environment",
			fileValue:   "file.db",
			environment: "environment.db",
			want: func(paths) expectation {
				return expectation{
					value:     "environment.db",
					effective: workingDirectoryPath(t, "environment.db"),
					source:    configloader.SourceEnv,
				}
			},
		},
		{
			name:        "flag",
			fileValue:   "file.db",
			environment: "environment.db",
			flag:        "flag.db",
			want: func(paths) expectation {
				return expectation{
					value:     "flag.db",
					effective: workingDirectoryPath(t, "flag.db"),
					source:    pflagloader.SourcePFlag,
				}
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

			loaded, err := config.Load("", false, flags)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			want := test.want(paths{config: configPath, data: dataHome})
			if loaded.Config.DBPath != want.value {
				t.Errorf("Load() Config.DBPath = %q, want loaded value %q", loaded.Config.DBPath, want.value)
			}
			if loaded.Effective.DBPath != want.effective {
				t.Errorf("Load() Effective.DBPath = %q, want %q", loaded.Effective.DBPath, want.effective)
			}
			if loaded.Report.Updates["dbpath"] != want.source {
				t.Errorf("Load() source = %q, want %q", loaded.Report.Updates["dbpath"], want.source)
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

	loaded, err := config.Load(explicit, true, newFlags(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Config.DBPath != "explicit.db" {
		t.Errorf("Load() Config.DBPath = %q, want explicit file value", loaded.Config.DBPath)
	}
	want := filepath.Join(filepath.Dir(explicit), "explicit.db")
	if loaded.Effective.DBPath != want {
		t.Errorf("Load() Effective.DBPath = %q, want %q", loaded.Effective.DBPath, want)
	}
	if len(loaded.Report.LoadedFiles) != 1 || loaded.Report.LoadedFiles[0] != explicit {
		t.Errorf("Load() files = %#v, want only %q", loaded.Report.LoadedFiles, explicit)
	}
}

func TestLoadResolvesRelativeExplicitFileAgainstWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GSD_DB", "")
	writeConfig(t, filepath.Join(dir, "settings", "config.toml"), "db_path = 'nested/gsd.db'\n")

	loaded, err := config.Load(filepath.Join("settings", "config.toml"), true, newFlags(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := filepath.Join(dir, "settings", "nested", "gsd.db")
	if loaded.Effective.DBPath != want {
		t.Errorf("Load() Effective.DBPath = %q, want absolute %q", loaded.Effective.DBPath, want)
	}
}

func TestLoadResolvesRelativeEnvironmentAndFlagPathsAgainstWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("GSD_DB", "environment.db")

	environment, err := config.Load("", false, newFlags(t))
	if err != nil {
		t.Fatalf("Load(environment) error = %v", err)
	}
	if environment.Config.DBPath != "environment.db" {
		t.Errorf("environment Config.DBPath = %q, want unchanged relative path", environment.Config.DBPath)
	}
	if want := workingDirectoryPath(t, "environment.db"); environment.Effective.DBPath != want {
		t.Errorf("environment Effective.DBPath = %q, want %q", environment.Effective.DBPath, want)
	}

	flag, err := config.Load("", false, newFlags(t, "--db", "flag.db"))
	if err != nil {
		t.Fatalf("Load(flag) error = %v", err)
	}
	if flag.Config.DBPath != "flag.db" {
		t.Errorf("flag Config.DBPath = %q, want unchanged relative path", flag.Config.DBPath)
	}
	if want := workingDirectoryPath(t, "flag.db"); flag.Effective.DBPath != want {
		t.Errorf("flag Effective.DBPath = %q, want %q", flag.Effective.DBPath, want)
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
	loaded, err := config.Load("", false, newFlags(t, "--db="))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := filepath.Join(filepath.Dir(configPath), "file.db")
	if loaded.Config.DBPath != "file.db" || loaded.Effective.DBPath != want ||
		loaded.Report.Updates["dbpath"] != configPath {
		t.Errorf("Load() = %#v; want file value resolving to %q from %q", loaded, want, configPath)
	}
}

func TestLoadDefaultsToHomeDataDirectoryWithoutXDGDataHome(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("GSD_DB", "")

	loaded, err := config.Load("", false, newFlags(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := filepath.Join(home, ".local", "share", "gsd", "gsd.db")
	if loaded.Config.DBPath != want || loaded.Effective.DBPath != want ||
		loaded.Report.Updates["dbpath"] != configloader.SourceDefault {
		t.Errorf("Load() = %#v; want default %q", loaded, want)
	}
}

func TestLoadLetsHigherPriorityOverridesReplaceEmptyFilePath(t *testing.T) {
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
			path := filepath.Join(dir, "config.toml")
			writeConfig(t, path, "db_path = ''\n")
			t.Setenv("HOME", filepath.Join(dir, "home"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			t.Setenv("GSD_DB", test.environment)

			loaded, err := config.Load(path, true, newFlags(t, test.flagArgs...))
			if err != nil {
				t.Fatalf("Load() error = %v, want the override to replace the empty file value", err)
			}
			if loaded.Config.DBPath != test.want || loaded.Report.Updates["dbpath"] != test.wantSource {
				t.Errorf("Load() = %#v; want %q from %q", loaded, test.want, test.wantSource)
			}
			if want := workingDirectoryPath(t, test.want); loaded.Effective.DBPath != want {
				t.Errorf("Load() Effective.DBPath = %q, want %q", loaded.Effective.DBPath, want)
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

	_, err := config.Load("", false, newFlags(t))
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

			loaded, err := config.Load("", false, newFlags(t, test.flagArgs...))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if loaded.Config.DBPath != test.want || loaded.Report.Updates["dbpath"] != test.wantSource {
				t.Errorf("Load() = %#v; want %q from %q", loaded, test.want, test.wantSource)
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

			_, err := config.Load(test.setup(t, dir), true, newFlags(t))
			if err == nil {
				t.Fatal("Load() error = nil, want invalid_argument")
			}
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.InvalidArgument {
				t.Errorf("Load() error = %v, want invalid_argument", err)
			}
			if !strings.HasPrefix(err.Error(), "invalid configuration:") {
				t.Errorf("Load() error = %q, want 'invalid configuration:' prefix", err)
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

			_, err := config.Load("", false, newFlags(t))
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

// workingDirectoryPath returns path joined onto the process working directory,
// the location a relative --db or GSD_DB value must resolve to.
func workingDirectoryPath(t *testing.T, path string) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	return filepath.Join(workingDirectory, path)
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
