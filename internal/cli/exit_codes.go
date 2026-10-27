package cli

import "github.com/spf13/cobra"

func exitCodesTopic() *cobra.Command {
	return newHelpTopic(
		"exit-codes",
		"Exit codes and failure reporting",
		`gsd exits 0, 1, or 2. main.go returns the status that the root adapter in
internal/cli/root.go picks from the error a command returns.

  0  Success, including a bare 'gsd', --help, --version, and 'gsd help
     NAME' for an unknown NAME (it prints 'Unknown help topic' and the
     root usage on stderr). Reported outcomes that are not failures also
     exit 0: an empty list ([] with --json), a tag already attached or
     already absent, 'done' on a promoting task whose project is
     already at its last stage, a 'project move' to the project's
     current stage, a tui view that fails to load (the error shows in
     place and the session continues), and a capture or tui session
     left with esc, q, or ctrl+c.
  1  Application error: a missing row, a bad ID, date, status, or search
     expression, an edit with no field flags, an invalid configuration,
     a state rule that blocks a change, or an unexpected failure such
     as an unreadable database. Bad IDs, bad statuses, and missing field
     flags are rejected before the database is opened.
  2  Usage error: an unknown command, flag, or operand, a wrong operand
     count, a flag needing a value, conflicting flags or a missing
     required flag group, --first=false or --last=false, a bare command
     group, --json on capture, config, or tui, and capture or tui
     without a terminal, with an unset or dumb TERM, or with operands.
     The database is never opened.

Every failure prints one 'gsd: <message>' line on stderr without usage
text and leaves stdout empty, also with --json. --json never changes the
exit status. This topic prints its text to stdout, needs no terminal or
database, and never prompts.`,
	)
}
