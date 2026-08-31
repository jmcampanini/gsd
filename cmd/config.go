package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/jmcampanini/gsd/internal/text"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type configurationLoader func(
	string,
	bool,
	*pflag.FlagSet,
) (config.Config, configloader.LoadReport, error)

func newConfigCommand(options *rootOptions, loadConfiguration configurationLoader) *cobra.Command {
	var showProvenance bool

	command := &cobra.Command{
		Use:   "config",
		Short: "Print the effective configuration",
		Long: `Print the effective configuration as redirectable TOML.

` + databaseDiscoveryHelp + `

The only key is db_path. The report is one line, db_path = "PATH", with
PATH made absolute, and it can be saved as a config file. Use
--provenance to add the source of each value as a TOML comment:
'default', 'env: GSD_DB', 'flag: --db', or 'file: PATH'. There are no
secret values to redact. An explicit --config file that is missing or
unreadable, a file that cannot be parsed, or an empty db_path in a file
is an invalid_argument error (exit 1) whose message starts with 'invalid
configuration:'. This command loads the configuration but never opens
the database, so it works before the database exists.

The global --json flag is not supported by this command because TOML is
its machine-readable format; giving it is a usage error (exit 2) raised
before the configuration is loaded. Output goes to stdout, errors go to
stderr as 'Error: <message>', and nothing prompts.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if options.json {
				return usageError("--json is not supported by gsd config; use the TOML output")
			}

			flags := command.Root().PersistentFlags()
			loaded, report, err := loadConfiguration(
				options.configPath,
				flags.Changed("config"),
				flags,
			)
			if err != nil {
				return normalizeApplicationError(err)
			}

			return normalizeApplicationError(writeConfigReport(
				command.OutOrStdout(),
				loaded,
				report,
				showProvenance,
			))
		},
	}
	command.Flags().BoolVar(&showProvenance, "provenance", false, "include field-level configuration provenance")
	return command
}

func writeConfigReport(
	writer io.Writer,
	loaded config.Config,
	report configloader.LoadReport,
	showProvenance bool,
) error {
	effectivePath, err := filepath.Abs(loaded.DBPath)
	if err != nil {
		return fmt.Errorf("resolve effective database path: %w", err)
	}
	loaded.DBPath = effectivePath

	reporter := configreporter.New(loaded, report)
	if !showProvenance {
		return reporter.WriteTOML(writer)
	}

	contents, err := reporter.TOML()
	if err != nil {
		return err
	}
	rows := reporter.ProvenanceRows()
	if len(rows) != 1 || len(rows[0]) != 3 || rows[0][0] != config.DBPathKey {
		return fmt.Errorf("config report has unexpected provenance shape")
	}

	line := strings.TrimSuffix(string(contents), "\n")
	if line == "" || strings.Contains(line, "\n") {
		return fmt.Errorf("config report has unexpected TOML shape")
	}
	_, err = fmt.Fprintf(writer, "%s # %s\n", line, configSourceDescription(rows[0][2]))
	return err
}

func configSourceDescription(source string) string {
	switch kind, path := config.ClassifySource(source); kind {
	case config.SourceKindEnv:
		return "env: GSD_DB"
	case config.SourceKindFlag:
		return "flag: --db"
	case config.SourceKindFile:
		return "file: " + text.Human(path, false)
	default:
		return "default"
	}
}
