# Command Spec (v1)

Three surfaces, one grammar. The CLI is the canonical surface; the TUI
embeds the same grammar; agents consume the CLI directly (`--json`) plus
raw SQL through `gsd query`. Full parity: every operation exists in both
human surfaces. The data contract behind `query` lives in `SCHEMA.md`.

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
gsd areas list [--archived]
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

- **Mutual exclusion**: `--project` and `--area` together is an error
  (matches the schema CHECK). Neither = inbox.
- **Re-parenting appends** to the end of the destination container.
- **Reorder is sibling-relative**; referencing an entity in a different
  container is an error.
- **Tags must pre-exist**: `tag`/`untag` with an unknown name is an error.
  `gsd tags add` is the only way tags come into existence.
- **Cascades narrate**: completing/cancelling a project cancels its open
  tasks and reports each one.
- **Delete honors RESTRICT**: deleting a non-empty project/area is an
  error; `--recursive` is the explicit opt-in (children deleted in one
  transaction). Deleting a task never blocks.
- **Dates**: canonical `YYYY-MM-DD`, plus a closed keyword set — `today`,
  `tomorrow`, weekday names (`mon`..`sun` = next occurrence), `+Nd`,
  `+Nw`. Nothing else parses.
- **Notes are markdown by convention** but never parsed or interpreted by
  any tool. `--note -` reads stdin.

## Output contract

- `--json` (global flag) makes any command machine-readable. An entity in
  JSON is its table row — same column names, same formats, including the
  derived `status` — plus `tags` as an array of names.
- **Mutations echo the affected entity**: `gsd add --json` returns the
  created row (agents capture the new ID without a second call).
- **Cascades report what they touched**:
  `{"project": {...}, "cancelled_tasks": [{...}, ...]}`.
- **Errors are structured, on stderr**:
  `{"error": {"code": "not_found", "message": "no task 42"}}`. Codes are
  stable API; messages are not.
- **Exit codes stay coarse**: `0` success, `1` domain error, `2` usage
  error. Fine distinctions live in the JSON error code.

## Database

Default path `$XDG_DATA_HOME/gsd/gsd.db`
(`~/.local/share/gsd/gsd.db`). Precedence: `--db PATH`, then `GSD_DB`,
then the default. No config file in v1.

## TUI

Full parity with the CLI, achieved structurally:

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
