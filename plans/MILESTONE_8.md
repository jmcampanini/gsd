# Milestone 8 — Search

Data mode: throwaway. Depends on: Milestone 7.

Written light; re-review at plan gate.

## Capability

Find the thing you half-remember: FTS5 full-text search over titles and
notes of all three entity kinds, composing with the list filters. The
FTS infrastructure lands while data is still throwaway — a
`user_version` bump (`9008`) folded into the Go-live baseline, not a
live migration.

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

0. **Milestone 7 consolidation** — record only: the Milestone 7
   foundation review scheduled no findings (see below).
1. **Index schema** — FTS table + sync, `user_version` bump, backfill
   verified against existing rows in a seeded dev db.
2. **Search command** — expression passthrough, filter composition,
   mixed-kind output.

### Chunk 0: Milestone 7 consolidation

The Milestone 7 foundation review scheduled no findings; there is no
chunk 0 work. The deliberately deferred items carry forward with their
revisit triggers:

- **Config report generalization** — on config key #2 (`[serve] addr`,
  arriving in Milestone 11): add source classification and tag-derived
  env/flag spellings to go-config-loader's `configreporter` so gsd's
  renderer becomes a generic provenance-row loop like the sibling CLIs,
  and revisit the reporting/redaction contract at the same moment. No
  load-request struct — positional load parameters are the family
  idiom.
- **Genericizing the intentionally-parallel tag service flows** —
  carried from Milestone 6: revisit on the first sibling-divergence bug
  or a post-v1 attach-semantics change. The reorder verb added a third
  concretely-parallel flow family; weighed against this trigger at the
  Milestone 7 foundation review, the convention held.
- **Typed transition spec for `applyTransition`** — carried from
  Milestone 6: revisit if post-v1 work adds transitions.

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

On a seeded dev database:

1. Schema applies cleanly; FTS rows == entity rows (backfill check).
2. Prefix, phrase, and OR queries return expected known-data hits across
   kinds; malformed expression → `invalid_argument`.
3. Edit a note, re-search: change reflected (sync works).
4. Filter composition matrix with `--status`, `--tag`, `--project`,
   `--area`.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)).

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003.
