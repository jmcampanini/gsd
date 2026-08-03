# Command Spec (v1)

The CLI is the canonical v1 surface; agents consume entity operations
through `--json`, configuration through TOML, plus raw SQL through `gsd
query`. A post-v1 TUI is planned to embed the same grammar and call the same
parser and core. The data contract behind `query` lives in `SCHEMA.md`. This
document specifies the canonical v1 target; the roadmap in `MILESTONES.md`
delivers it incrementally.

## Grammar

- **Verb-first; bare verbs act on tasks.** `gsd add`, `gsd done 42`.
  Tasks are the default noun and elide entirely.
- **Plural noun = operate on the collection** (create into it, enumerate
  it): `gsd projects add`, `gsd areas list`. Bare plurals are an error —
  `list` is always explicit.
- **Singular noun = operate on one entity, always by ID**: `gsd project
  done 7`, `gsd area archive 2`.
- **Bare integer IDs; the noun disambiguates.** `gsd done 42` is a task,
  `gsd project done 7` is a project. Mixed output (logbook, search)
  prints the kind next to the ID.
- **Tags are addressed by name**, never ID — the name is the identity.
  All tag administration lives on the plural (`tags`); singular `tag` /
  `untag` are verbs that attach/detach.
- **One obvious way.** No aliases, no sugar verbs, no second spellings.
- **Same field, same flag, everywhere.** `add` and `edit` share one flag
  vocabulary; `--no-<field>` clears an optional field (`--no-due`,
  `--no-project`). There is no `move`: `edit` owns re-parenting.
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
                    [--tag NAME]... [--note TEXT|-]
gsd list [--project N] [--area N] [--tag NAME]
          [--status open|done|cancelled|all]        # default: open
          [--due] [--overdue] [--deferred]
gsd show N
gsd edit N [--title TEXT] [--note TEXT|-]
            [--due DATE | --no-due] [--defer DATE | --no-defer]
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
gsd projects add "Kitchen reno" [--area N] [--tag NAME]... [--note TEXT|-]
gsd projects list [--area N] [--status open|done|cancelled|all]
gsd project show N
gsd project edit N [--title TEXT] [--note TEXT|-]
                    [--area N | --no-area]
gsd project done N          # cancels remaining open tasks; reports them
gsd project cancel N        # same cascade
gsd project reopen N
gsd project tag N NAME...
gsd project untag N NAME...
gsd project reorder N (--after M | --before M | --first | --last)
gsd project delete N [--recursive]
```

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

## Search and query

```
gsd search "EXPR" [--project N] [--area N] [--tag NAME] [--status ...]
gsd query "SELECT ..."      # or "-" to read SQL from stdin
```

- `search` passes `EXPR` through FTS5 match syntax (`plumb*`, `"exact
  phrase"`, `a OR b`) over titles and notes, all entity kinds, and
  composes with the `list` filter flags. The FTS index is internal — not
  part of the query contract in v1.
- `query` runs on a read-only connection: SELECT-only by construction.
  Human output is an aligned table of the selected columns; `--json` is
  an array of row objects.

## Semantics

- **Mutual exclusion**: `--project` and `--area` together is
  `invalid_argument` (matches the schema CHECK), on task `add`, `edit`, and
  as `list` filters. Pairing a set flag with its own clear
  (`--area N --no-area`) is a usage error. Neither = inbox.
- **Re-parenting appends** to the end of the destination container;
  re-stating the current container is a no-op and does not move the entity.
- **Reorder is sibling-relative**; referencing an entity in a different
  container is an error.
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
  reopening its tasks, creating a task into it, and re-parenting a task into
  or out of it are `conflict` errors with reopen-the-project-first guidance
  (a move blocked by a resolved source and a resolved destination names
  both). Content edits and deletion of contained tasks stay allowed.
- **An archived area is retired history**, mirroring the resolved-project
  guard through the governing area (own, or inherited through the project):
  creating a task or project into it, re-parenting into or out of it, and
  completing, cancelling, or reopening anything it governs are `conflict`
  errors with unarchive-the-area-first guidance; a move blocked by more than
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
- **Dates**: canonical `YYYY-MM-DD`, plus a closed keyword set — `today`,
  `tomorrow`, weekday names (`mon`..`sun` = next occurrence), `+Nd`,
  `+Nw`. Nothing else parses.
- **Date filters are mutually exclusive**: `--due` selects tasks in the chosen
  status whose due date is set; `--overdue` selects only open tasks due before
  the local calendar day; `--deferred` selects tasks in the chosen status
  deferred beyond the local calendar day.
- **Notes are markdown by convention** but never parsed or interpreted by
  any tool. `--note ""` clears a note. `--note -` reads stdin through EOF
  without stripping trailing newlines.
- **Validation is semantic, not lossy.** IDs are positive decimals; titles are
  valid UTF-8 and nonblank after surrounding-space inspection, but accepted
  text is stored unchanged. Notes must be valid UTF-8. Semantic failures use
  `invalid_argument`; command grammar and flag failures use `usage`.

## Output contract

- `--json` is a global persistent complete-output-mode flag for commands that
  support JSON. `gsd config` is the one exception: combining it with
  `--json` is a usage error (exit `2`), and TOML is its machine-readable
  format. Successful entity output is its table row — the same column names
  and formats, including derived `status` — plus `tags`, an array of stored
  tag names in alphabetical
  (`NOCASE`) order, matching `tags list`.
  The complete v1 entity field sets are:
  - task: `id`, `project_id`, `area_id`, `title`, `note`, `defer_until`,
    `due_on`, `done_at`, `cancelled_at`, `status`, `position`, `created_at`,
    `updated_at`, `tags`;
  - project: `id`, `area_id`, `title`, `note`, `done_at`, `cancelled_at`,
    `status`, `position`, `created_at`, `updated_at`, `tags`;
  - area: `id`, `title`, `note`, `archived_at`, `position`, `created_at`,
    `updated_at`, `tags`.
  Nullable values are `null`; collections are arrays, including `[]`.
- JSON output is exactly one compact value followed by a newline. Tags
  complete the v1 entity field set, but field order remains unstable; field
  names, types, and error codes are stable, while message wording is not.
- **Mutations echo the affected entity**: the three `add` commands and all
  `tag`/`untag` commands return the complete entity row, so agents can capture
  an ID or the resulting tags without another call.
- **Tag administration returns complete rows and envelopes**: a tag row is
  `{"id":N,"title":"...","created_at":"...","updated_at":"..."}`;
  `tags add` and `tags rename` return that row. `tags list` returns an array
  of rows with `id`, `title`, `created_at`, `updated_at`, and `usage_count`,
  totaled across tasks, projects, and areas. `tags delete` returns
  `{"tag":{...},"detached":N}`, where `tag` is the deleted row and `detached`
  is the total removed attachment count.
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
  field/value outline beneath a status-glyph headline, mutations use concise
  glyph-prefixed action payloads, and `tags list` prints `#`-prefixed stored
  names with cross-entity usage counts, ordered alphabetically with `NOCASE`,
  without IDs or a header. Task, project, and area `show` include a `tags`
  row of `#`-prefixed stored names, blank when untagged; collection rows
  gain no tags column.
- Successful tag mutation lines are concise and action-prefixed:
  `Added tag NAME`, `Renamed tag OLD to NEW`,
  `Deleted tag NAME (detached from N items)`, `Tagged: KIND ID  NAME`, and
  `Untagged: KIND ID  NAME`, each behind its verb-class glyph. They use
  stored spelling after case-insensitive resolution; rename prints both the
  stored previous and new titles.
- Styling grammar: faint marks metadata (IDs, kinds, counts, timestamps,
  headers, non-urgent dates), bold marks urgency (due today or overdue),
  glyphs mark records and events (`+` add, `−` delete, `✓` done, `✗`
  cancel, `#` tag verbs and tag names, `•` other mutations and open
  records, `└` cascade children), and hue marks state change only — green
  for done/added, red for cancelled/deleted/urgent — drawn from Catppuccin
  Latte on light terminal backgrounds and Frappé on dark, accents-only.
  Identity markers and metadata stay monochrome; stderr diagnostics are
  unstyled in v1. Structure (headers, glyphs, layout) is identical across
  color modes; modes control only ANSI styling.
- Bare `gsd tags`, `gsd tag`, and `gsd untag` are usage errors (exit `2`) and
  do not open the database.
- Human output escapes ASCII control characters (`show` preserves note line
  breaks) so stored text cannot inject terminal control sequences.

## Configuration

The Config milestone implements this canonical v1 contract; specifying it here
before implementation keeps the v1 target authoritative while earlier
milestones use the narrower baseline behavior.

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
Parent
directories are created when opening the database. During throwaway-data
milestones, only a genuinely empty version-0 database is bootstrapped; a
nonempty version-0 or differently
versioned database fails with `conflict` and delete-your-dev-db guidance.

## TUI (post-v1)

The planned TUI targets full parity with the CLI, achieved structurally:

- **`:` opens a command line that accepts the CLI grammar verbatim**,
  minus the binary name (`:projects add "Kitchen reno" --area 3`). It
  calls the same parser and core — parity is shared code, not discipline.
- **Single-key bindings are mnemonics for the same verbs**, applied to
  the selected row: `a` add, `d` done, `x` cancel, `e` edit, `t` tag,
  `o` reopen, `D` delete (with confirm). Reordering is grab-and-move on
  the selection. Keys are shorthand for verbs, never a second vocabulary.
- **Three panes, Things-style**: sidebar (Inbox, Available, Logbook, then
  areas with their projects nested), list pane, and a toggleable detail
  pane rendering `show` (note as markdown, tags, dates).
- **`/` is incremental search** over the same FTS index as `gsd search`,
  filtering the current view live.
