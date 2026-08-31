package cmd

import "github.com/spf13/cobra"

func exitCodesTopic() *cobra.Command {
	return &cobra.Command{
		Use:   "exit-codes",
		Short: "Exit codes and error categories",
		Long: `gsd exits 0, 1, or 2. main.go returns the status that the root adapter in
cmd/root.go picks from the error a command returns.

  0  Success, including a bare 'gsd', --help, --version, and 'gsd help
     NAME' for an unknown NAME (it prints the root help). Reported
     outcomes that are not failures also exit 0: an empty list ([] with
     --json), a tag already attached or already absent, 'done' on a
     promoting task whose project is already at its last stage, a
     'project move' to the project's current stage, and a capture or
     tui session left with esc, q, or ctrl+c.
  1  Application error. The categories are not_found (no row for an ID
     or name), invalid_argument (a bad ID, date, status, or search
     expression, an edit with no field flags, or an invalid
     configuration), conflict (a state rule, such as completing a task
     twice, deleting an occupied project or area without --recursive,
     a duplicate name, or a resolved project or archived area blocking
     a change), and internal (an unexpected failure such as an
     unreadable database). 'Error: <message>' goes to stderr, or with
     --json one line {"error":{"code":CODE,"message":TEXT}}; stdout
     stays empty. Bad IDs, bad statuses, and missing field flags are
     rejected before the database is opened.
  2  Usage error: an unknown command, flag, or operand, a wrong operand
     count, a flag needing a value, conflicting flags or a missing
     required flag group, --first=false or --last=false, a bare command
     group, --json on capture, config, or tui, and capture or tui
     without a terminal or with operands. 'Error: <message>' goes to
     stderr even with --json, stdout stays empty, and the database is
     never opened.

--json never changes the exit status.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}
