# Milestone 9 — Search

Data mode: live. Depends on: Milestone 8.

Written light; re-review at plan gate (see note in `MILESTONE_8.md`).

## Capability

Find the thing you half-remember: FTS5 full-text search over titles and
notes of all three entity kinds, composing with the list filters. Also the
first **live schema migration** — the FTS infrastructure ships as
migration 0002+, proving the Milestone 7 machinery on real data.

## Scope

- FTS5 index over `title` + `note` of tasks, projects, areas, kept in
  sync (triggers or equivalent — implementation detail). The index is
  internal: not part of the `query` contract (COMMANDS.md § "Search and
  query"), so its shape can change freely later.
- Command:

```text
gsd search "EXPR" [--project N] [--area N] [--tag NAME] [--status ...]
```

- `EXPR` passes through FTS5 match syntax: `plumb*`, `"exact phrase"`,
  `a OR b`. Malformed FTS expressions are `invalid_argument`, not a
  panic.
- Mixed-kind output prints the kind next to the ID (like `logbook`).
  Filters compose: kinds that a filter can't apply to are excluded when
  that filter is present (**proposed**: `--project N` restricts results
  to tasks in that project).

## Chunks

1. **Index migration** — FTS table + sync, migration applied to a copy
   of the live db, backfill verified against existing rows.
2. **Search command** — expression passthrough, filter composition,
   mixed-kind output.

## User stories

### Half a memory is enough

```text
$ gsd search "plumb*"
  task     4   Call plumber
  project  11  Bathroom plumbing
$ gsd search '"pick up" OR errand' --status open
```

### Search narrows like list does

```text
$ gsd search tile --project 11 --status done
  task  23  Pick tiles   done
```

## Agent-verified end-to-end workflow

On a copy of the live database:

1. Migration applies cleanly; FTS rows == entity rows (backfill check).
2. Prefix, phrase, and OR queries return expected known-data hits across
   kinds; malformed expression → `invalid_argument`.
3. Edit a note, re-search: change reflected (sync works).
4. Filter composition matrix with `--status`, `--tag`, `--project`,
   `--area`.
5. On the real db: Javier searches for something he actually
   half-remembers; it comes back.

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] Live db migrated with zero data loss (row counts by kind unchanged).

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003.
