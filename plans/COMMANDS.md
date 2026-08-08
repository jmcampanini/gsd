# Command Spec (v1)

The CLI is the canonical v1 surface; agents consume entity operations
through `--json` and configuration through TOML. A post-v1 TUI is arriving
incrementally — `gsd capture` is its first shipped surface — and will
embed the same grammar and call the same parser and core. This document
specifies the canonical v1 target; the roadmap in `MILESTONES.md` delivers
it incrementally.

## Grammar

- **Verb-first; bare verbs act on tasks.** `gsd add`, `gsd done 42`.
  Tasks are the default noun and elide entirely.
- **Plural noun = operate on the collection** (create into it, enumerate
  it): `gsd projects add`, `gsd areas list`. Bare plurals are an error —
  `list` is always explicit.
- **Singular noun = operate on one entity.** Its argument is that entity's
  identity: an ID for ID-addressed entities (`gsd project done 7`) and a name
  for name-addressed entities (`gsd board show software`).
- **Bare integer IDs; the noun disambiguates.** `gsd done 42` is a task,
  `gsd project done 7` is a project. Mixed output (logbook, search)
  prints the kind next to the ID.
- **Tags are addressed by name**, never ID — the name is the identity.
  All tag administration lives on the plural (`tags`); singular `tag` /
  `untag` are verbs that attach/detach.
- **One obvious way.** No aliases, no sugar verbs, no second spellings.
- **Same field, same flag, everywhere.** `add` and `edit` share one flag
  vocabulary; `--no-<field>` clears an optional field (`--no-due`,
  `--no-project`). `edit` owns containment re-parenting; the dedicated
  `project move` verb owns movement on the board axis.
- **One entity per invocation.** No multi-add; bulk is a later decision.

## Views

```
gsd inbox         # open tasks, no project/area; position order
gsd available     # workable right now; see SCHEMA.md for the definition
gsd logbook       # done + cancelled, tasks & projects; newest first
```

## Tasks (default noun)

```
gsd add "Buy milk" [--project N | --area N] [--due DATE] [--defer DATE]
                    [--defer-stage STAGE | --no-defer-stage]
                    [--promotes | --no-promotes]
                    [--tag NAME]... [--note TEXT|-]
gsd list [--project N] [--area N] [--tag NAME]
          [--status open|done|cancelled|all]        # default: open
          [--due] [--overdue] [--deferred]
gsd show N
gsd edit N [--title TEXT] [--note TEXT|-]
            [--due DATE | --no-due] [--defer DATE | --no-defer]
            [--defer-stage STAGE | --no-defer-stage]
            [--promotes | --no-promotes]
            [--project N | --no-project] [--area N | --no-area]
gsd done N
gsd cancel N
gsd reopen N
gsd tag N NAME...
gsd untag N NAME...
gsd reorder N (--after M | --before M | --first | --last)
gsd delete N
```

## Projects

```
gsd projects add "Kitchen reno" [--area N] [--board NAME]
                 [--tag NAME]... [--note TEXT|-]
gsd projects list [--area N] [--status open|done|cancelled|all]
gsd project show N
gsd project edit N [--title TEXT] [--note TEXT|-]
                    [--area N | --no-area] [--board NAME | --no-board]
gsd project done N          # cancels remaining open tasks; reports them
gsd project cancel N        # same cascade
gsd project reopen N
gsd project tag N NAME...
gsd project untag N NAME...
gsd project reorder N (--after M | --before M | --first | --last)
gsd project move N STAGE [--after M | --before M | --first | --last]
gsd project delete N [--recursive]
```

## Boards and stages

```
gsd boards add NAME --stage NAME [--stage NAME ...] [--note TEXT|-]
gsd boards list
gsd board show NAME
gsd board edit NAME [--title TEXT] [--note TEXT|-]
gsd board reorder NAME (--after NAME | --before NAME | --first | --last)
gsd board delete NAME
gsd stages add BOARD NAME [--after STAGE | --before STAGE | --first | --last]
gsd stage rename BOARD OLD NEW
gsd stage reorder BOARD NAME (--after STAGE | --before STAGE | --first | --last)
gsd stage delete BOARD NAME
```

Boards are global, ordered pipelines orthogonal to areas. A board has a note
and one or more named stages at creation (`boards add` without `--stage` is
`invalid_argument`); later stage deletion may leave it empty. Board names are
globally `NOCASE`-unique, and stage names are `NOCASE`-unique within a board;
accepted spelling is stored unchanged and lookups are case-insensitive under
SQLite `NOCASE`.

A project belongs to at most one board and occupies one stage. Board entry
always uses the first stage; switching boards re-enters the new first stage,
and `--no-board` removes membership. Entry onto a board with no stages is a
`conflict`. Board containment is independent of area containment, but its
changes inherit the resolved-project and archived-area guards.

`project move` owns the board axis. A cross-stage move appends unless an
explicit placement positions it among projects in the destination stage; a
same-stage placement reorders that column, while a bare same-stage move is a
no-op that still narrates `~ Moved:`. Moving a project that is on no board is
a `conflict`. Movement may go in either direction, does not enforce sequence,
and is not gated by tasks. It is a transition verb guarded like one: moving a
resolved project, or any project under an archived governing area, is a
`conflict`, checked before the same-stage no-op. Placement references are
project IDs in the destination stage, including resolved projects that
retain hidden positions. Resolving a
project hides it from `board show`; reopening restores it at the same stage
and position. Deleting an occupied board or stage is a `conflict`; deleting
an empty board deletes its stages.

## Areas

```
gsd areas add "Home" [--tag NAME]... [--note TEXT|-]
gsd areas list [--archived | --all]
gsd area show N
gsd area edit N [--title TEXT] [--note TEXT|-]
gsd area archive N
gsd area unarchive N
gsd area tag N NAME...
gsd area untag N NAME...
gsd area reorder N (--after M | --before M | --first | --last)
gsd area delete N [--recursive]
```

## Tags

```
gsd tags add NAME
gsd tags list               # includes usage counts
gsd tags rename OLD NEW
gsd tags delete NAME        # detaches from everything
```

## Search

```
gsd search "EXPR" [--related]
```

- `search` passes `EXPR` through FTS5 match syntax (`plumb*`, `"exact
  phrase"`, `a OR b`) over the title, tags, and note of every task,
  project, and area — all statuses, archived areas included. `--related`
  widens the same search through inherited context (a container's title
  and tags), ranking every direct match above every context-only match;
  within a tier, relevance orders results, with ties broken by kind
  (task, project, area) then id. A blank or malformed expression is
  `invalid_argument`. Human rows show kind, id, title, status, and the
  container-title context path; `--json` is an array of
  kind-discriminated complete entity rows in relevance order. The FTS
  index is internal and virtual — built per invocation, nothing
  persists, results always reflect current data.

## Capture

```
gsd capture
```

- `capture` is the first TUI surface: a popup-sized single-input
  program built to live inside `tmux display-popup` (recommended
  invocation: `tmux display-popup -w 64 -h 4 -E 'gsd capture'`). It
  runs in the terminal's alternate screen and is keyboard-only.
- Enter with a non-blank title creates an open task with exactly
  `gsd add TITLE` semantics — the title verbatim, untrimmed, no other
  fields — then exits `0` with no output; the popup just vanishes.
  Enter on blank input (empty or whitespace-only) is a no-op. Esc and
  Ctrl+C cancel: exit `0`, nothing written. A cancel during an
  in-flight write waits for the write to settle, and a write that wins
  the cancellation race keeps the normal success path.
- Interactive-only: `capture` takes no arguments, and `--json` and
  non-TTY invocation (checked per stream) are usage errors (exit `2`)
  whose messages name `gsd add` as the noninteractive path.
- The full configuration chain applies (`--config`, `--db`, `GSD_DB`,
  TOML, defaults); the database opens only when the program starts, so
  help and parse failures never touch it.
- A failed write renders its application error inline behind the red
  accent; any key dismisses, and the command exits `1` with the
  standard stderr diagnostic.
- Chrome: a `gsd` badge as the input prompt, a continuous input band,
  and a faint `enter add · esc cancel` footer that reports transient
  `adding` / `canceling` statuses while an in-flight write settles.
  Color follows
  CLI-OUTPUT-001 (`--color` > nonempty `NO_COLOR` > per-stream auto);
  light/dark accents are detected in-program from the terminal
  background, the cursor stays terminal-default, and structure is
  identical uncolored.
- The capture e2e drives a real tmux session, making tmux a
  development and CI prerequisite (documented in the README).

## Semantics

- **Mutual exclusion**: `--project` and `--area` together is
  `invalid_argument` (matches the schema CHECK), on task `add`, `edit`, and
  as `list` filters. Pairing a set flag with its own clear
  (`--area N --no-area`, `--defer-stage S --no-defer-stage`,
  `--promotes --no-promotes`, or `--board B --no-board`) is a usage error.
  On task `add` and `edit`, boolean marker and clear flags must be
  meaningfully true; explicitly false values are usage errors rather than
  alternate spellings.
  Neither task container = inbox.
- **Re-parenting appends** to the end of the destination container;
  re-stating the current container is a no-op and does not move the entity.
- **Reorder is sibling-relative**: the reference entity must live in the
  same container as the moved one; a cross-container reference or a
  self-reference is `invalid_argument`. Reordering is status-blind — done,
  cancelled, and archived siblings keep their positions, can be moved, and
  can serve as references. The board axis applies the same reference rule:
  resolved projects retain hidden stage positions and remain valid placement
  references, though `move` itself carries the transition guards — a resolved
  project, or one under an archived governing area, cannot move. A placement
  that lands the entity where it already sits succeeds as an ordinary
  reorder. Exactly one placement flag must be meaningfully given. An
  explicitly false boolean placement (`--first=false`) is the same arity
  failure as omitting placement
  entirely, a usage error.
- **Tag names use title validation**: they must be valid UTF-8 and nonblank
  after surrounding-space inspection, and accepted spelling is stored
  unchanged. The stored spelling (initially the first-created spelling) is
  displayed. Uniqueness and every name lookup use SQLite `NOCASE`, which
  folds ASCII only; non-ASCII case variants remain distinct. Conflicts name
  the existing stored spelling. A case-only rename of the same tag succeeds;
  renaming onto another existing tag is `conflict`.
- **Tags must pre-exist**: unknown names on `tag`, `untag`, repeated `--tag`
  on task/project/area `add`, or task `list --tag` are `not_found`
  (`no tag NAME`). `gsd tags add` is the only creation path. Attach, detach, filter,
  rename-source, and delete-target matching all resolve to the stored
  spelling case-insensitively.
- **Attach and detach are idempotent**: attaching an already attached tag or
  detaching an existing but unattached tag succeeds without changing the
  join rows. Multi-name `tag`/`untag` and repeated `add --tag` operations are
  all-or-nothing; an unknown name changes nothing, and duplicate names,
  including ASCII case variants, collapse to one attachment. Tagged creation
  is transactional, so a failed tag lookup leaves no new entity.
- **Tagging is an unguarded content edit**: attach and detach remain allowed
  under archived governing areas and resolved projects. They change only
  join rows and never bump the entity's `updated_at`; rename bumps the tag's
  own `updated_at`.
- **Rename and deletion propagate through joins**: renaming immediately
  changes every attached entity's `tags` value. Deleting a tag detaches it
  everywhere and reports the total count; deleting a tagged entity removes
  its join rows and reduces that tag's usage count.
- **Cascades narrate**: completing/cancelling a project cancels its open
  tasks and reports each one.
- **Reopening a project is not un-cascade**: it clears only the project's
  exit; tasks the cascade cancelled stay cancelled until individually
  reopened.
- **A resolved project is closed history**: completing, cancelling, or
  reopening its tasks, creating a task into it, re-parenting a task into or
  out of it, and moving it on the board axis are `conflict` errors with
  reopen-the-project-first guidance
  (a move blocked by a resolved source and a resolved destination names
  both). Content edits and deletion of contained tasks stay allowed.
- **An archived area is retired history**, mirroring the resolved-project
  guard through the governing area (own, or inherited through the project):
  creating a task or project into it, re-parenting into or out of it, moving
  a project it governs on the board axis, and completing, cancelling, or
  reopening anything it governs are `conflict` errors with
  unarchive-the-area-first guidance; a move blocked by more than
  one guard names every blocker. Content edits and deletes stay allowed,
  archiving never mutates contents, and unarchive restores visibility with
  every position intact.
- **`areas list` partitions by archive state**: archived areas are hidden by
  default, `--archived` lists only archived ones, and `--all` lists both
  with an archived marker on the archived rows. The two flags are mutually
  exclusive.
- **Referencing a nonexistent project or area is `not_found`** everywhere
  its ID can appear: `--project` and `--area` on task `add`, `edit`, and
  `list`; `projects add --area`, `projects list --area`, and
  `project edit --area`; and every singular `project` or `area` verb.
- **Delete honors RESTRICT**: deleting a non-empty project/area is an
  error; `--recursive` is the explicit opt-in (children deleted in one
  transaction). Deleting a task never blocks.
- **Stage defer**: `--defer-stage` requires the task's destination project
  to be on a board and the named stage to belong to that board. Unknown names
  are `not_found`; an off-board project or a stage from another board is
  `invalid_argument`. On a combined task re-parent plus explicit stage defer,
  the destination is resolved first and the defer is validated against that
  destination. A stage defer clears automatically when the task is
  re-parented away, when its project leaves or switches boards, or when the
  referenced stage is deleted. The containment edit or deletion and all
  clears are one operation.
- **Promotion**: `--promotes` is valid on any task and is inert if its project
  has no board. Completing a promoting task advances its project exactly one
  stage from its current position and appends it to the destination column in
  the same operation. At the last stage this succeeds as a reported no-op;
  reopening the task never moves the project backward.
- **Stage and date gates are independent**: a task is available only after
  both its date defer has arrived and its project has reached or passed its
  deferred stage by stage position. Moving the project backward can hide it
  again. `list --deferred` includes tasks blocked by either gate.
- **Dates**: canonical `YYYY-MM-DD`, plus a closed keyword set — `today`,
  `tomorrow`, weekday names (`mon`..`sun` = next occurrence), `+Nd`,
  `+Nw`. Nothing else parses.
- **Date/defer filters are mutually exclusive**: `--due` selects tasks in the
  chosen status whose due date is set; `--overdue` selects only open tasks due
  before the local calendar day; `--deferred` selects tasks in the chosen
  status blocked by a future defer date or an unreached defer stage.
- **Notes are markdown by convention** but never parsed or interpreted by
  any tool. `--note ""` clears a note. `--note -` reads stdin through EOF
  without stripping trailing newlines.
- **Validation is semantic, not lossy.** IDs are positive decimals; titles are
  valid UTF-8 and nonblank after surrounding-space inspection, but accepted
  text is stored unchanged. Notes must be valid UTF-8. Semantic failures use
  `invalid_argument`; command grammar and flag failures use `usage`.

## Output contract

- `--json` is a global persistent complete-output-mode flag for commands that
  support JSON. `gsd config` and `gsd capture` are the exceptions:
  combining either with `--json` is a usage error (exit `2`) — TOML is
  `config`'s machine-readable format, and `capture` is
  interactive-only. Successful entity output is its table row — the same column names
  and formats, including derived `status` — plus `tags`, an array of stored
  tag names in alphabetical
  (`NOCASE`) order, matching `tags list`.
  The complete v1 entity field sets are:
  - task: `id`, `project_id`, `area_id`, `title`, `note`, `defer_until`,
    `due_on`, `done_at`, `cancelled_at`, `status`, `position`, `created_at`,
    `updated_at`, `defer_stage_id`, `promotes`, `tags`;
  - project: `id`, `area_id`, `title`, `note`, `done_at`, `cancelled_at`,
    `status`, `position`, `created_at`, `updated_at`, `stage_id`,
    `stage_position`, `tags`;
  - area: `id`, `title`, `note`, `archived_at`, `position`, `created_at`,
    `updated_at`, `tags`;
  - board: `id`, `title`, `note`, `position`, `created_at`, `updated_at`;
  - stage: `id`, `board_id`, `title`, `position`, `created_at`, `updated_at`.
  Nullable values are `null`; collections are arrays, including `[]`.
  `promotes` is a JSON boolean although SQLite stores its constrained integer
  representation. Resolved board and stage names are presentation metadata:
  task JSON exposes `defer_stage_id` but not `defer_stage_title`, and project
  JSON exposes `stage_id` and `stage_position` but not board or stage titles.
  View-task JSON still includes its established project and governing-area
  enrichment.
- JSON output is exactly one compact value followed by a newline. Tags
  complete the v1 entity field set, but field order remains unstable; field
  names, types, and error codes are stable, while message wording is not.
- **Mutations echo the affected entity**: the three `add` commands, all
  `tag`/`untag` commands, and the three `reorder` commands return the
  complete entity row, so agents can capture an ID, the resulting tags, or
  the new `position` without another call. An agent wanting the container's
  resulting order follows with a position-ordered collection command.
- **Tag administration returns complete rows and envelopes**: a tag row is
  `{"id":N,"title":"...","created_at":"...","updated_at":"..."}`;
  `tags add` and `tags rename` return that row. `tags list` returns an array
  of rows with `id`, `title`, `created_at`, `updated_at`, and `usage_count`,
  totaled across tasks, projects, and areas. `tags delete` returns
  `{"tag":{...},"detached":N}`, where `tag` is the deleted row and `detached`
  is the total removed attachment count.
- **Board envelopes name every affected row.** `boards list` returns board
  rows each with an ordered `stages` array. `board show` returns
  `{"board":{...},"stages":[{...stage row...,"projects":[{...project row...,"progress":{"done":N,"total":M}}]}]}`;
  cancelled tasks count in neither progress value. `board delete` returns
  `{"board":{...},"stages":[...]}`. Every other board and stage mutation
  echoes its bare affected row: `boards add`, `board edit`, and
  `board reorder` return the board row (an agent wanting the resulting
  stages follows with `boards list`); `stages add`, `stage rename`, and
  `stage reorder` return the stage row. A project board edit returns
  `{"project":{...},"cleared_defers":[...]}`. A task containment edit
  returns `{"task":{...},"cleared_defers":[...]}`, and stage deletion returns
  `{"stage":{...},"cleared_defers":[...]}`. These `cleared_defers` values
  are always arrays, including when empty. For a task containment edit, the
  array reports the stage defer cleared by an actual re-parent, including when
  `--no-defer-stage` also requested that clear. Completing a promoting
  task returns `{"task":{...},"promoted_project":{...}}`;
  `promoted_project` is `null` when no project moves because the task has no
  project, the project is off-board, or it is already at the last stage.
- **Cascades report what they touched**:
  `{"project":{...},"cancelled_tasks":[{...},...]}`; recursive deletion
  mirrors it as `{"project":{...},"deleted_tasks":[...]}` and, for areas,
  `{"area":{...},"deleted_projects":[...],"deleted_tasks":[...]}` —
  `deleted_tasks` grouped by container: the area's loose tasks first,
  then each deleted project's tasks following `deleted_projects` order,
  with every group (and `deleted_projects` itself) ordered by
  `position`, then `id`. All arrays may be `[]`.
- **Collections are position-ordered**: `inbox` and `list` return rows
  ordered by `position`, then `id`, for every status filter, in both output
  modes.
- **The logbook is resolution-ordered**: `resolved_at` descending, project
  entries above task entries resolved at the same instant, then `id`
  descending, so a cascade lists the project above the tasks it cancelled.
  Human rows print kind, ID, title, status, and the local calendar day of
  resolution.
- **Every JSON-mode application error is structured, on stderr**:
  `{"error":{"code":"not_found","message":"no task 42"}}`. Initial stable
  codes are `not_found`, `invalid_argument`, `conflict`, and `internal`;
  `internal` messages carry the underlying diagnostic. Command grammar and
  flag failures stay human-readable on stderr even under `--json` — exit
  code `2` is their machine signal. Default mode keeps human-readable
  stderr diagnostics for every error.
- **Exit codes stay coarse**: `0` success, `1` application error, `2` usage
  error. Fine distinctions live in the JSON error code.
- Human collections are aligned tables with lowercase faint column headers,
  rendered only when rows exist; empty collections print nothing. `show` is a
  field/value outline beneath a status-glyph headline: `•` open task, `◆` open
  project, `●` active area, `✓` done task/project, and `✗` cancelled
  task/project or archived area. Mutations use concise glyph-prefixed action
  payloads, and `tags list` prints `#`-prefixed stored names with cross-entity
  usage counts, ordered alphabetically with `NOCASE`, without IDs or a header.
  Task, project, and area `show` include a `tags` row of `#`-prefixed stored
  names, blank when untagged; project `show` also names `board/stage`, and task
  `show` names its defer stage and promoting intent. Collection rows gain no
  tags column. A promoting task carries a faint `↑` beside its title wherever
  the title renders, in listings and in the `show` headline alike.
- Board mutations use `+ Board: NAME (stage → stage)`, `~ Edited:`,
  `~ Moved:`, and `~ Promoted:` lines. Stage administration uses
  `+ Added stage BOARD/STAGE`, `~ Renamed stage BOARD/OLD to BOARD/NEW`,
  `~ Reordered: stage BOARD/STAGE`, and `− Deleted: stage BOARD/STAGE`.
  Automatic defer clears are narrated as `├ Cleared stage defer:` /
  `└ Cleared stage defer:` children of the containment change or stage
  deletion. `boards list` prints each board with its ordered stage path;
  `board show` prints every column, including `(empty)`, without truncation,
  and heads a board with no stages `(no stages)`.
- Successful tag mutation lines are concise and action-prefixed:
  `Added tag NAME`, `Renamed tag OLD to NEW`,
  `Deleted tag NAME (detached from N items)`, `Tagged: KIND ID  #NAME`, and
  `Untagged: KIND ID  #NAME`, each behind its verb-class glyph. Tagging uses
  `+#`; untagging uses `−#`. They use stored spelling after case-insensitive
  resolution; rename prints both the stored previous and new titles.
- Styling grammar: faint marks metadata (IDs, kinds, counts, timestamps,
  headers, non-urgent dates), bold marks urgency (due today or overdue on open
  tasks), and glyphs mark records and events. Events use `+` add, `−` delete,
  `✓` done, `✗` cancel/archive, `+#` tag, `−#` untag, and `~` neutral
  mutations. Record glyphs follow the `show` mapping above; cascade children
  use `├` until the final `└`. Hue marks state change only — green for
  done/added, red for cancelled/archived/deleted/urgent — drawn from
  Catppuccin Latte on light terminal backgrounds and Frappé on dark,
  accents-only. Identity markers, structural glyphs, and metadata stay
  monochrome; stderr diagnostics are unstyled in v1. Structure (headers,
  glyphs, layout) is identical across color modes; modes control only ANSI
  styling.
- Bare `gsd tags`, `gsd tag`, and `gsd untag` are usage errors (exit `2`) and
  do not open the database.
- Human output escapes ASCII control characters (`show` preserves note line
  breaks) so stored text cannot inject terminal control sequences.

## Configuration

- The discovered config file is TOML at
  `$XDG_CONFIG_HOME/gsd/config.toml`. It is optional. When `--config PATH` is
  given, that exact file is required: a missing, unreadable, or invalid file
  fails rather than falling back to discovery. Under go-config-loader's
  current behavior, a missing discovered path or a directory at that path is
  treated as absent, and a relative `XDG_CONFIG_HOME` is resolved from the
  working directory; this remains subject to go-config-loader issue #13. A
  discovered file that exists but cannot load is `invalid_argument`, as is a
  file-provided empty `db_path`, even when `GSD_DB` or `--db` supplies a valid
  path. Issue #13 tracks both this strict contract and the alternative of
  letting higher-precedence sources patch over a broken discovered file.
- The only v1 key is `db_path`. New keys are permanent API and require a
  demonstrated need. Color is deliberately not a configuration key.
- `gsd config` prints valid, redirectable TOML for the effective config;
  TOML is this command's machine-readable format. It renders `db_path` as the
  absolute effective runtime location so relative environment and flag values
  round-trip from any snapshot directory. `gsd config --provenance` keeps the
  report valid TOML by adding an inline source comment normalized
  to `default`, `file: PATH`, `env: GSD_DB`, or `flag: --db`. Despite the
  inherited global flag, `gsd config --json` is a usage error (exit `2`).
- `db_path` is intentionally reportable and non-sensitive, so v1 has no
  redaction code or hooks. The reporting contract must be revisited before
  adding any future sensitive key.
- Color accepts `--color=auto|always|never` with an explicit value.
  Resolution is the explicit `--color` flag, then nonempty `NO_COLOR`, then
  destination-aware `auto`, evaluated per output stream (CLI-OUTPUT-001/002).
  Auto disables color for non-terminals and `TERM=dumb`; `FORCE_COLOR`,
  `CLICOLOR`, and `CLICOLOR_FORCE` are not consulted. JSON output never
  contains ANSI sequences, including under `--color=always`.

## Database

The default path is `$XDG_DATA_HOME/gsd/gsd.db`, falling back to
`~/.local/share/gsd/gsd.db`. Precedence is `--db PATH`, then nonempty `GSD_DB`,
then config-file `db_path`, then the default. Empty `GSD_DB` and explicit
`--db ""` fall through instead of overriding a lower-precedence value; this is
a database-path compatibility rule, not a general contract for future config
keys. Relative file values are resolved from the config file's directory;
relative env and flag values are resolved from the working directory. The
`gsd config` report uses the corresponding absolute runtime location so
redirected TOML preserves that location when reloaded from another directory.
Parent directories are created when opening the database.

Schema migrations apply automatically and silently when a behavioral
command opens the database; help, version, and argument parsing never
touch it. `PRAGMA application_id` identifies gsd files and
`PRAGMA user_version` tracks the applied revision. The guard ladder on
open (all refusals are `conflict`-coded, exit 1):

- A database at the current revision opens normally; one at an earlier
  gsd revision applies the pending migrations in sequence, then opens.
- A genuinely empty version-0 database receives the identity-stamped
  baseline and the full migration chain.
- A nonempty version-0 database is refused as foreign:
  `database is not empty; delete your development database and try
  again`.
- A database with any other application identity is refused before any
  DDL, file untouched: `database does not belong to gsd`.
- A gsd database newer than the binary is refused:
  `gsd is older than this database (database revision N, this gsd
  supports up to M); upgrade gsd`.
- A gsd database stamped with a negative revision is refused as
  corrupt: `database revision N is invalid`.

## TUI (post-v1)

The TUI arrives as Milestones 10 and 12–15 and targets full parity
with the CLI, achieved structurally. Milestone 10 shipped the
substrate and the first surface; the rest remains planned:

- **Full-screen views, no panes**: exactly one view at a time — a root
  tree (Inbox, Available, Logbook, loose projects, then areas with
  their open projects nested), container lists, and a detail view
  rendering `show`. Every container list carries a compact selectable
  header for the container itself; opening it shows that container's
  detail. Navigation replaces the whole screen, so the same structure
  works in a full terminal and a tmux popup. Pane and split layouts,
  mouse support, and markdown note rendering are parked explorations.
- **`gsd capture` shipped as the first surface** — specified in the
  Capture section above. It later grows inline syntax and a
  command-runner mode.
- **`:` opens a command line that accepts the CLI grammar verbatim**,
  minus the binary name (`:projects add "Kitchen reno" --area 3`). It
  calls the same parser and core — parity is shared code, not discipline.
- **Single-key bindings are mnemonics for the same verbs**, applied to
  the selected row: `a` add, `d` done, `x` cancel, `e` edit, `t` tag,
  `o` reopen, `D` delete (with confirm). Reordering is grab-and-move on
  the selection. Keys are shorthand for verbs, never a second vocabulary.
- **`/` is incremental search** with the same semantics as `gsd search`,
  filtering the current view live.
