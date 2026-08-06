# Milestone 9 — Go live

Data mode: **live from here on**. Depends on: Milestone 8.

## Capability

gsd stops being a project and becomes the system of record: schema
migrations make the database durable. From here `SCHEMA.md`'s stability
contract is in force, every future schema change ships as a numbered
migration, and real data enters through daily use.

## Scope

### Migration runner

- Numbered, embedded SQL migrations; `PRAGMA user_version` tracks the
  applied revision; applied automatically on open, each migration in its
  own transaction.
- `0001_baseline.sql` = the accumulated throwaway schema through Search
  — tags, the alphabetical tag-array ordering — verbatim from the
  schema-convergence audit (i.e., `SCHEMA.md`; the FTS index is virtual
  and persists nothing). v1 ships no live migrations; the runner exists
  for everything after.
- A database newer than the binary (`user_version` > known max, below
  the dev-only range) is a fail-loud error ("gsd is older than this
  database").
- Pre-baseline throwaway dbs are refused by name: their `user_version`
  sits in the dev-only range (`9000 + roadmap milestone number`), which can
  never collide with migration numbers, and the runner answers it with the
  delete-your-dev-db message. They hold throwaway data by declaration.
- From here, every schema change ships as a new migration file, and
  `SCHEMA.md`'s stability contract is in force: columns/tables are
  add-only, views only gain columns.

## Chunks

1. **Migration runner** — runner + baseline + guards + tests (fresh db,
   sequential apply, mid-migration failure rolls back, future version
   refused, dev-range stamp refused). Single chunk: the milestone is one
   vertical capability, the durable-database lifecycle.

## Carried from Milestone 8

The Milestone 8 foundation review's fix-now findings were resolved in
its wrap-up; there is no chunk 0 work. The deliberately deferred items
carry forward with their revisit triggers:

- **Config report generalization** — on config key #2 (`[serve] addr`,
  arriving with the optional Serve milestone, 14): add source
  classification and tag-derived env/flag spellings to
  go-config-loader's `configreporter` so gsd's renderer becomes a
  generic provenance-row loop like the sibling CLIs, and revisit the
  reporting/redaction contract at the same moment. No load-request
  struct — positional load parameters are the family idiom.
- **Genericizing the intentionally-parallel tag service flows** —
  carried from Milestone 6: revisit on the first sibling-divergence bug
  or a post-v1 attach-semantics change.
- **Typed transition spec for `applyTransition`** — carried from
  Milestone 6: revisit if post-v1 work adds transitions.
- **`search.Hit` constructors and accessors** — the hand-rolled sum
  type's invariant (exactly one entity pointer, matching `Kind`) is
  enforced at its consumers: revisit on the first new `Hit` consumer or
  producer, expected at the TUI milestones.
- **Entity-plus-container-titles projection consolidation** — logbook
  entries, task views, and search hits each assemble container-title
  context concretely in parallel, per convention: revisit on the fourth
  projection or the first context-inconsistency bug between surfaces.
- **In-expression scoping operators** (`in:`, `is:`, `~stem`/trigram
  markers) — parked: revisit when unfiltered search proves too broad in
  daily use; the spellings are reserved by FTS5 rejection today, and
  the virtual index makes alternate tokenizers a per-invocation swap.
- **Embeddings / semantic search** — parked, post-v1: revisit if
  tag-based topical search (`--related`) proves insufficient in daily
  use; the realistic path is an optional local-encoder sidecar fused
  with FTS, and nothing in Search forecloses it.
- **bm25 weight tuning** — the 4/3/2/1 values are a starting point:
  revisit after real-data use; tests pin ordering properties only, so a
  retune is a one-line change.

## User stories

### Your data survives upgrades by design

```text
$ brew upgrade --fetch-HEAD gsd    # after any future milestone
$ gsd inbox                        # migrations applied silently; data intact
```

## Agent-verified end-to-end workflow

1. Migration tests inside `make check` (fresh apply, sequential apply,
   mid-migration failure rollback, future-version refusal, dev-range
   refusal).
2. The agent drives the real built binary: a fresh database opens at
   the baseline revision and data persists across invocations; a
   dev-range database is refused with the delete-your-dev-db message; a
   future-revision database is refused with "gsd is older than this
   database".

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md` § Database reconciled to live-era semantics: an
      empty version-0 database receives the baseline migration;
      dev-range and future revisions fail loud with their exact
      messages.
- [ ] The temporary `DIVERGENCES.md` intake is empty; canonical specs
      describe the shipped system exactly.

## Standards

CLI-QUALITY-001.
