package cmd

// Shared help fragments compose command long descriptions so repeated
// contract text cannot drift between commands. Fragments carry no leading or
// trailing newline; compose them with explicit separators.

const outputContractHelp = `Output goes to stdout. Errors go to stderr as 'Error: <message>' and leave
stdout empty. Nothing prompts. --json writes one JSON value on a single
line to stdout, newline-terminated and free of terminal escapes; an
application error then goes to stderr as one line
{"error":{"code":CODE,"message":TEXT}}, where CODE is not_found,
invalid_argument, conflict, or internal, while usage errors stay
human-readable. The exit status is the same with and without --json.`

const groupContractHelp = `The group itself does nothing: running it without a subcommand, or with a
name that is not a subcommand, is a usage error (exit 2). Each
subcommand's --help states its own contract.`

const idGrammarHelp = `ID is the positive decimal integer shown in list and show output. Any other
value is an invalid_argument error (exit 1) raised before the database is
opened.`

const nameGrammarHelp = `Names are matched case-insensitively against stored titles and must not be
blank: a blank name is invalid_argument and an unknown name is not_found
(both exit 1).`

const databaseDiscoveryHelp = `The database path comes from the highest layer that sets it: the --db flag,
then the GSD_DB environment variable, then db_path in the config file,
then the default $XDG_DATA_HOME/gsd/gsd.db, or ~/.local/share/gsd/gsd.db
when XDG_DATA_HOME is unset. Empty --db and GSD_DB values are ignored. The
config file is --config PATH, which must exist, or otherwise
$XDG_CONFIG_HOME/gsd/config.toml (~/.config/gsd/config.toml when
XDG_CONFIG_HOME is unset), which may be absent. A relative db_path in the
file resolves against the file's directory; a relative --db or GSD_DB
resolves against the working directory. Every command that reads or
changes data opens the database, creating its directory and file when
missing and applying schema migrations; help, version, config, and
argument checks never open it.`

const dateGrammarHelp = `DATE is YYYY-MM-DD, today, tomorrow, a weekday abbreviation (sun, mon, tue,
wed, thu, fri, or sat) meaning the next such day after today, or +Nd or
+Nw for N days or weeks from today. Relative forms use the local calendar
date at invocation, and every date is stored and printed as YYYY-MM-DD.
Any other value is an invalid_argument error (exit 1).`

const placementHelp = `--first, --last, --after ID, and --before ID choose the new position and
are mutually exclusive; --first=false and --last=false are usage errors
(exit 2). Siblings are renumbered from zero in the new order. A reference
that is the row itself or is not a sibling is invalid_argument and an
unknown reference is not_found (both exit 1).`

const namedPlacementHelp = `--first, --last, --after NAME, and --before NAME choose the new position
and are mutually exclusive; --first=false and --last=false are usage
errors (exit 2). Siblings are renumbered from zero in the new order. A
reference that is the row itself is invalid_argument and an unknown
reference is not_found (both exit 1).`

const statusFilterHelp = `status is open, done, or cancelled. --status STATUS keeps one status, or
every status with all; the default is open. Any other value is
invalid_argument (exit 1) raised before the database is opened.`

const noteFlagHelp = `--note TEXT sets the note, and --note - reads the whole of stdin as the
note instead. A note may be empty and must be valid UTF-8; a stdin read
failure is an internal error (exit 1).`

const tagFlagHelp = `--tag NAME attaches an existing tag and may repeat; each value is one whole
name, so commas are not separators. Names match case-insensitively and
duplicates collapse to the first spelling. An unknown tag is not_found
(exit 1), and then nothing is created.`

const taggingContractHelp = `A name already attached stays attached and a name not attached is left
alone, so repeating either command changes nothing. Names match
case-insensitively, duplicates in one command collapse to the first
spelling, a blank name is invalid_argument, and an unknown tag is
not_found; either error exits 1 and changes nothing. Tags are never
created here; use 'gsd tags add NAME' first.

` + idGrammarHelp + `

On success one line names the row and the tags in their stored spelling;
with --json the updated row, including its full tags list, is written.

` + outputContractHelp

const taskContainerHelp = `A task lives in one container: one project, one area, or neither, which is
the inbox. --project ID and --area ID cannot be combined (invalid_argument,
exit 1), an unknown container is not_found (exit 1), and a task is placed
last in the container it enters.`

const blockerGuidanceHelp = `A resolved project or an archived area blocks changes to what it contains.
The conflict message (exit 1) names the blockers and ends with the commands
to run first, in the form 'gsd project reopen ID' or 'gsd area unarchive
ID'.`

const terminalContractHelp = `Stdin and stdout must both be terminals and --json is not accepted; each
violation is a usage error (exit 2) whose message names the noninteractive
alternative, raised before the database is opened. The session runs in the
terminal's alternate screen, which is restored on exit. Color follows the
--color rules in 'gsd --help', and when color is on the terminal background
is queried once to choose a light or dark palette.`
