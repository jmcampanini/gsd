# Milestone 9 — Go live

Data mode: **live from here on**. Depends on: Milestone 8.

## Capability

gsd stops being a project and becomes the system of record: schema
migrations make the database durable. From here `SCHEMA.md`'s stability
contract is in force, every future schema change ships as a numbered
migration, and real data enters through daily use.

## Scope

### Migration runner

- Numbered, embedded SQL migrations; `PRAGMA application_id` identifies
  gsd files and `PRAGMA user_version` tracks the applied revision. Both
  are stamped inside the migration transaction; migrations apply
  automatically on open.
- `0001_baseline.sql` = the accumulated throwaway schema through Search
  — tags, the alphabetical tag-array ordering — verbatim from the
  schema-convergence audit (i.e., `SCHEMA.md`; the FTS index is virtual
  and persists nothing). v1 ships no live migrations; the runner exists
  for everything after.
- A database without gsd's application identity is refused before DDL.
  A gsd database newer than the binary (`user_version` > known max) is a
  fail-loud error ("gsd is older than this database").
- Pre-baseline throwaway dbs get no special case: the dev-range stamps
  (`9000 + roadmap milestone number`) are never met in the wild, so a
  leftover simply fails the newer-database guard. Only a genuinely
  empty version-0 database receives the baseline; a nonempty version-0
  file is refused as foreign.
- From here, every schema change ships as a new migration file, and
  `SCHEMA.md`'s stability contract is in force as
  **additive-or-full-delete**: a surviving table or view only gains
  appended columns — never renames, retypes, or losses — and removal is
  only a whole-object drop. A lint test enforces the contract as an
  end-state schema diff across the migration chain; indexes are exempt.

## Chunks

1. **Migration runner** — runner + baseline + identity and revision
   guards + contract lint + tests (fresh db, sequential apply,
   mid-migration failure rolls back, foreign and future versions refused,
   nonempty version-0 refused, additive-or-full-delete lint). Single
   chunk: the milestone is one vertical capability, the durable-database
   lifecycle.

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
   mid-migration failure rollback, foreign-database refusal,
   future-version refusal, nonempty version-0 refusal,
   additive-or-full-delete lint).
2. The agent drives the real built binary: a fresh database opens at
   the baseline revision with gsd's application identity and data
   persists across invocations; a foreign database at a supported
   revision remains untouched; a future gsd revision is refused with
   "gsd is older than this database"; a nonempty version-0 file is
   refused as foreign.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [x] `COMMANDS.md` § Database reconciled to live-era semantics: an
      empty version-0 database receives the identity-stamped baseline;
      foreign identities, nonempty version-0, and future revisions fail
      loud with their exact messages. No canonical document still claims
      a dev-range guard exists.
- [x] `SCHEMA.md`'s stability contract amended to
      additive-or-full-delete, with the lint test named as its
      enforcement.
- [x] The temporary `DIVERGENCES.md` intake is empty; canonical specs
      describe the shipped system exactly.

## Standards

CLI-QUALITY-001.
