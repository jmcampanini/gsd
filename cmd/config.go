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

Use --provenance to add the source of each value as a TOML comment. The global
--json flag is not supported by this command because TOML is its machine-readable
format.`,
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
