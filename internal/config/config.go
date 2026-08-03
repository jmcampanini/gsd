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

const dbPathConfigKey = "dbpath"

type Config struct {
	DBPath string `toml:"db_path" config:"db" help:"path to the SQLite database"`
}

func RegisterFlags(flags *pflag.FlagSet) error {
	return pflagloader.Register[Config](flags)
}

func Load(
	configPath string,
	configPathExplicit bool,
	flags *pflag.FlagSet,
) (Config, configloader.LoadReport, error) {
	defaults, defaultError := defaultConfig()

	fileLoader, err := newFileLoader(configPath, configPathExplicit)
	if err != nil {
		if configPathExplicit {
			return Config{}, configloader.LoadReport{}, invalidConfig(err)
		}
		return Config{}, configloader.LoadReport{}, err
	}
	environmentLoader, err := configloader.NewEnvironmentLoader[Config]("gsd", nonemptyEnvironment())
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}

	loaded, report, err := configloader.Load(
		defaults,
		validatedFileLoader(fileLoader),
		environmentLoader,
		ignoreEmptyDBFlag(flags, flagLoader),
	)
	if err != nil {
		return Config{}, configloader.LoadReport{}, err
	}
	if loaded.DBPath == "" {
		if report.Updates[dbPathConfigKey] == configloader.SourceDefault && defaultError != nil {
			return Config{}, configloader.LoadReport{}, defaultError
		}
		return Config{}, configloader.LoadReport{}, invalidEmptyDBPath()
	}

	loaded.DBPath = resolveFileRelativeDBPath(loaded.DBPath, report.Updates[dbPathConfigKey])
	return loaded, report, nil
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

func validatedFileLoader(
	loader configloader.ConfigLoader[Config],
) configloader.ConfigLoader[Config] {
	return func(base Config) (Config, configloader.LoadReport, error) {
		loaded, report, err := loader(base)
		if err != nil {
			return base, configloader.LoadReport{}, invalidConfig(err)
		}
		if _, updated := report.Updates[dbPathConfigKey]; updated && loaded.DBPath == "" {
			return base, configloader.LoadReport{}, invalidEmptyDBPath()
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
			delete(report.Updates, dbPathConfigKey)
		}
		return loaded, report, nil
	}
}

func resolveFileRelativeDBPath(path, source string) string {
	if filepath.IsAbs(path) || !sourceIsFile(source) {
		return path
	}
	return filepath.Join(filepath.Dir(source), path)
}

func sourceIsFile(source string) bool {
	switch source {
	case "", configloader.SourceDefault, configloader.SourceEnv, pflagloader.SourcePFlag:
		return false
	default:
		return true
	}
}

func invalidEmptyDBPath() error {
	return invalidConfig(fmt.Errorf("database path is empty"))
}

func invalidConfig(err error) error {
	return apperr.New(apperr.InvalidArgument, "invalid configuration: "+err.Error(), err)
}
