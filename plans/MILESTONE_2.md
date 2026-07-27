# Milestone 2 — Time

Data mode: throwaway. Depends on: Milestone 1.

## Capability

Tasks gain calendar behavior: defer noise until it's relevant, mark real
external deadlines, and work from `available` — the "what could I do right
now" view. This completes single-entity task semantics before any
relationships arrive.

## Schema delta

- `tasks` gains `defer_until` and `due_on` with the round-trip validity
  CHECKs from `SCHEMA.md` (`defer_until IS date(defer_until)`, same for
  `due_on`).
- `available` view, reduced form: open tasks whose `defer_until` is empty
  or `<= date('now','localtime')`. Project/area clauses and enrichment
  columns arrive with their tables (Milestones 3–5).
- `user_version` bump (throwaway guard).

## Commands

```text
gsd add "TITLE" [--due DATE] [--defer DATE] ...
gsd edit N [--due DATE | --no-due] [--defer DATE | --no-defer] ...
gsd available
gsd list [--due] [--overdue] [--deferred] ...
```

Date grammar (closed set, COMMANDS.md § Semantics): canonical
`YYYY-MM-DD`, `today`, `tomorrow`, weekday names (`mon`..`sun` = next
occurrence), `+Nd`, `+Nw`. Nothing else parses — garbage and non-canonical
forms are `invalid_argument`. All resolution against the local calendar
day.

## Proposed defaults (flag in review if wrong)

- `--overdue` = open tasks with `due_on < today`; `--due` = any due date
  set; `--deferred` = `defer_until > today` (list shows deferred tasks;
  only `available` hides them).
- Human list output shows due/defer compactly; overdue rows styled when
  color is on. JSON stays bare column values (`YYYY-MM-DD`).

## Chunks

1. **Date engine + fields** — the keyword parser as a pure, exhaustively
   unit-tested package (`internal/dates`), schema delta, `--due`/`--defer`
   on `add`/`edit`, `--no-due`/`--no-defer` clearing. The parser is a
   guardrail showcase: closed grammar, table-driven tests, rejection
   cases.
2. **Views and filters** — `available` view + command, `list` filter
   flags, overdue presentation.

## User stories

### You can silence a task until it matters

```text
$ gsd add "Book campsite" --defer sat
$ gsd available        # Tuesday: not listed
$ gsd list --deferred
  4  Book campsite   defer 2026-08-01
```

### Real deadlines surface instead of drowning

```text
$ gsd add "File taxes" --due 2026-10-15
$ gsd list --overdue   # nothing, until it's late
```

### The date grammar refuses to guess

```text
$ gsd add "Bad date" --defer 2026-2-30
{"error": {"code": "invalid_argument", "message": "not a date: 2026-2-30"}}
```

## Agent-verified end-to-end workflow

Fresh temp db, `--json`:

1. Add tasks with `--defer today`, `--defer tomorrow`, `--defer +1w`,
   `--due today`; assert stored values are canonical dates.
2. `available` contains the `today` deferral, excludes `tomorrow`
   and `+1w`.
3. `edit --no-defer` on the excluded task; it appears in `available`.
4. `list --overdue` empty; add `--due` yesterday via canonical date;
   assert it appears.
5. Reject matrix: `2026-02-30`, `2026-8-3`, `next tuesday`, `+3x` — all
   `invalid_argument`, exit 1.
6. Weekday keyword resolves to the next occurrence (computed
   independently by the agent from today's date).

## Exit criteria

Standard exit criteria (see `MILESTONES.md`).

## Standards

CLI-CMD-002/003, CLI-OUTPUT-002/003.
