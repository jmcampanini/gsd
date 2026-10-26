// Package config loads the database path from the XDG default, the config
// file, the GSD_DB variable, and the --db flag, reports which layer supplied
// it, and derives the absolute path commands open.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	configloader "github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/spf13/pflag"
)

// DBPathKey is the load-report key go-config-loader derives for DBPath.
const DBPathKey = "dbpath"

// SourceKind classifies a load-report source value.
type SourceKind int

// The source kinds a load report can name.
const (
	SourceKindDefault SourceKind = iota
	SourceKindEnv
	SourceKindFlag
	SourceKindFile
)

// ClassifySource maps a load-report source to its kind. For SourceKindFile the
// second result is the config file path that supplied the value.
func ClassifySource(source string) (SourceKind, string) {
	switch source {
	case configloader.SourceEnv:
		return SourceKindEnv, ""
	case pflagloader.SourcePFlag:
		return SourceKindFlag, ""
	case "", configloader.SourceDefault:
		return SourceKindDefault, ""
	default:
		return SourceKindFile, source
	}
}

// Config is the configuration as loaded, before derivation. DBPath is the
// SQLite database path exactly as the winning layer supplied it, bound to the
// db flag, the GSD_DB variable, and the db_path file key.
type Config struct {
	DBPath string `toml:"db_path" config:"db" help:"path to the SQLite database"`
}

// Effective holds the values derived from Config that commands consume.
type Effective struct {
	// DBPath is the absolute SQLite database path commands open.
	DBPath string
}

// Loaded is the result of Load: the configuration as loaded, the values
// derived from it, and the report naming the source of each loaded value.
type Loaded struct {
	Config    Config
	Effective Effective
	Report    configloader.LoadReport
}

// RegisterFlags adds the flags derived from Config, currently --db, to flags.
func RegisterFlags(flags *pflag.FlagSet) error {
	return pflagloader.Register[Config](flags)
}

// Load applies the file, environment, and flag layers to the defaults, then
// validates and derives the effective values once from the result. An
// explicit configPath must exist; otherwise the XDG config file may be
// absent. Empty --db and GSD_DB values are ignored. A relative db_path from a
// file resolves against that file's directory and a relative --db or GSD_DB
// against the working directory. An empty path after every layer has been
// applied is an invalid_argument error.
func Load(
	configPath string,
	configPathExplicit bool,
	flags *pflag.FlagSet,
) (Loaded, error) {
	defaults, defaultError := defaultConfig()

	fileLoader, err := newFileLoader(configPath, configPathExplicit)
	if err != nil {
		if configPathExplicit {
			return Loaded{}, invalidConfig(err)
		}
		return Loaded{}, err
	}
	environmentLoader, err := configloader.NewEnvironmentLoader[Config]("gsd", nonemptyEnvironment())
	if err != nil {
		return Loaded{}, err
	}
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	if err != nil {
		return Loaded{}, err
	}

	loaded, report, err := configloader.Load(
		defaults,
		invalidConfigFileLoader(fileLoader),
		environmentLoader,
		ignoreEmptyDBFlag(flags, flagLoader),
	)
	if err != nil {
		return Loaded{}, err
	}

	effective, err := derive(loaded, report.Updates[DBPathKey], defaultError)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Config: loaded, Effective: effective, Report: report}, nil
}

func defaultConfig() (Config, error) {
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return Config{DBPath: filepath.Join(dataHome, "gsd", "gsd.db")}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}
	return Config{DBPath: filepath.Join(home, ".local", "share", "gsd", "gsd.db")}, nil
}

func newFileLoader(
	configPath string,
	explicit bool,
) (configloader.ConfigLoader[Config], error) {
	if explicit {
		return configloader.NewRequiredFileLoader[Config](configPath)
	}

	helper, err := configloader.NewFileHelper("gsd", "config.toml")
	if err != nil {
		return nil, err
	}
	return configloader.NewMergeAllFilesLoader[Config](helper.XDGConfigFile())
}

func nonemptyEnvironment() map[string]string {
	environment := configloader.OSEnv()
	if environment["GSD_DB"] == "" {
		delete(environment, "GSD_DB")
	}
	return environment
}

func invalidConfigFileLoader(
	loader configloader.ConfigLoader[Config],
) configloader.ConfigLoader[Config] {
	return func(base Config) (Config, configloader.LoadReport, error) {
		loaded, report, err := loader(base)
		if err != nil {
			return base, configloader.LoadReport{}, invalidConfig(err)
		}
		return loaded, report, nil
	}
}

func ignoreEmptyDBFlag(
	flags *pflag.FlagSet,
	loader configloader.ConfigLoader[Config],
) configloader.ConfigLoader[Config] {
	return func(base Config) (Config, configloader.LoadReport, error) {
		loaded, report, err := loader(base)
		if err != nil {
			return base, configloader.LoadReport{}, err
		}
		flag := flags.Lookup("db")
		if flag != nil && flag.Changed && flag.Value.String() == "" {
			loaded.DBPath = base.DBPath
			delete(report.Updates, DBPathKey)
		}
		return loaded, report, nil
	}
}

// derive validates loaded once every layer has been applied and resolves its
// database path to an absolute path. source is the load-report source of
// DBPath; defaultError is the failure, if any, that left the default empty.
func derive(loaded Config, source string, defaultError error) (Effective, error) {
	kind, filePath := ClassifySource(source)
	if loaded.DBPath == "" {
		if kind == SourceKindDefault && defaultError != nil {
			return Effective{}, defaultError
		}
		return Effective{}, invalidConfig(fmt.Errorf("database path is empty"))
	}

	path := loaded.DBPath
	if kind == SourceKindFile && !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(filePath), path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Effective{}, fmt.Errorf("resolve database path %q: %w", path, err)
	}
	return Effective{DBPath: absolute}, nil
}

func invalidConfig(err error) error {
	return apperr.New(apperr.InvalidArgument, "invalid configuration: "+err.Error(), err)
}
