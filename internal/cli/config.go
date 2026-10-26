package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/jmcampanini/gsd/internal/clierror"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/config"
	"github.com/jmcampanini/gsd/internal/text"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type configurationLoader func(string, bool, *pflag.FlagSet) (config.Loaded, error)

func newConfigCommand(options *rootOptions, loadConfiguration configurationLoader) *cobra.Command {
	var showProvenance bool

	command := &cobra.Command{
		Use:   "config",
		Short: "Print the effective configuration",
		Long: `Print the effective configuration as redirectable TOML.

` + clihelp.DatabaseDiscoveryHelp + `

The only key is db_path. The report body is one line, db_path = "PATH",
with PATH exactly as the winning layer supplied it, so the output saved
as a config file loads the same value. A blank line and an '# Effective'
comment block follow; its effective_db_path line names the absolute
database path every other command opens, resolved as described above.
--provenance appends a '# Provenance' comment table with Field, Value,
and Source columns, where Source is 'default', 'env: GSD_DB',
'flag: --db', or 'file: PATH'. There are no secret values to redact. An
explicit --config file that is missing or unreadable, a file that cannot
be parsed, or an empty db_path in the file that no GSD_DB or --db value
replaces is an invalid_argument error (exit 1) whose message starts with
'invalid configuration:'. This command loads the configuration but never
opens the database, so it works before the database exists.

The global --json flag is not supported by this command because TOML is
its machine-readable format; giving it is a usage error (exit 2) raised
before the configuration is loaded. Output goes to stdout, errors go to
stderr as 'Error: <message>', and nothing prompts.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if options.json {
				return clierror.Usage("--json is not supported by gsd config; use the TOML output")
			}

			flags := command.Root().PersistentFlags()
			loaded, err := loadConfiguration(options.configPath, flags.Changed("config"), flags)
			if err != nil {
				return clierror.NormalizeApplication(err)
			}

			return clierror.NormalizeApplication(writeConfigReport(
				command.OutOrStdout(),
				loaded,
				showProvenance,
			))
		},
	}
	command.Flags().BoolVar(&showProvenance, "provenance", false, "include field-level configuration provenance")
	return command
}

// writeConfigReport writes the loaded configuration as TOML, then the
// '# Effective' comment block, then the '# Provenance' comment table when
// showProvenance is set.
func writeConfigReport(writer io.Writer, loaded config.Loaded, showProvenance bool) error {
	if err := configreporter.New(loaded.Config, loaded.Report).WriteTOML(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"\n# Effective\n# effective_db_path = %q\n",
		loaded.Effective.DBPath,
	); err != nil {
		return err
	}
	if !showProvenance {
		return nil
	}

	if _, err := fmt.Fprintln(writer, "\n# Provenance"); err != nil {
		return err
	}
	table := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "# Field\tValue\tSource"); err != nil {
		return err
	}
	source := configSourceDescription(loaded.Report.Updates[config.DBPathKey])
	if _, err := fmt.Fprintf(table, "# db_path\t%q\t%s\n", loaded.Config.DBPath, source); err != nil {
		return err
	}
	return table.Flush()
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
