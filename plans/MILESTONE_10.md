# Milestone 10 — Go live

Data mode: **live from here on**. Depends on: Milestone 8.

## Capability

gsd stops being a project and becomes the system of record: schema
migrations make the database durable, the install story makes the binary a
resident of the machine, and an agent-driven import moves real data in.
Exit condition is not a feature — it's "Javier lives in gsd."

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

### Install story

- Homebrew HEAD-only formula: shipped early (2026-08-06, PR #53) —
  `Formula/gsd.rb` with commit-derived version stamping, generated
  shell completions, a noninteractive functional test
  (CLI-RELEASE-003), and self-tap install/upgrade instructions in the
  README.
- README rewritten as landing page: purpose, `--HEAD` install/upgrade,
  representative commands, config discovery (CLI-DOCS-001/002).
- Shell completion documented — generation already ships with the
  formula, and gsd now has the interactive command surface that
  warrants it (CLI-CMD-005).

### Import (no gsd code)

- Export data from the current tool (format: whatever it exports —
  decided at import time).
- A Claude session drives the installed `gsd` binary: `tags add` →
  `areas add` → `projects add` → `add`, wiring IDs from `--json` echoes,
  setting defer/due dates and tags as they were.
- The agent then verifies: entity counts by kind match the export, spot
  checks on titles/notes/dates/tags, `available`/`inbox` look right.
- Transcript and count summary saved to `.sandbox/import/` and linked
  from the PR that closes the milestone.

## Chunks

1. **Migration runner** — runner + baseline + newer-db guard + tests
   (fresh db, sequential apply, mid-migration failure rolls back, future
   version refused).
2. **Install story** — README landing page and completion docs on top
   of the already-shipped formula; `brew install --HEAD` from the real
   tap verified on this machine.
3. **Import session** — no code: the agent-driven import itself,
   run against the real config-file-pinned database.

## Carried from Milestone 8

The Milestone 8 foundation review's fix-now findings were resolved in
its wrap-up; there is no chunk 0 work. The deliberately deferred items
carry forward with their revisit triggers:

- **Config report generalization** — on config key #2 (`[serve] addr`,
  arriving in Milestone 11): add source classification and tag-derived
  env/flag spellings to go-config-loader's `configreporter` so gsd's
  renderer becomes a generic provenance-row loop like the sibling CLIs,
  and revisit the reporting/redaction contract at the same moment. No
  load-request struct — positional load parameters are the family
  idiom.
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

### gsd is installed like software, not run like a script

```text
$ brew install jmcampanini/gsd/gsd --HEAD
$ gsd --version
gsd version v0.0.0-89-gdeadbee
```

### Your actual life is in it

```text
$ gsd areas list
  1  Home
  2  Work
$ gsd available
  ... the real things you could actually do today ...
```

### Your data survives upgrades by design

```text
$ brew upgrade --fetch-HEAD gsd    # after any future milestone
$ gsd inbox                        # migrations applied silently; data intact
```

## Agent-verified end-to-end workflow

1. Migration tests inside `make check` (fresh apply, idempotent reopen,
   failure rollback, future-version refusal).
2. `brew install --HEAD` from the tap; `gsd --version` reports commit
   identity; formula's functional test passes.
3. The import session itself — the largest agent-verified workflow yet:
   every entity kind created through the public CLI, counts verified
   against the source export.
4. Post-import: `gsd available`, `inbox`, `logbook` each return sensible
   real data (reviewed by Javier, not asserted by the agent).

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] **Functional completeness audit** (carried from the cancelled Query
      milestone): every command and behavior `COMMANDS.md` promises
      (excluding the forward-looking TUI section) is demonstrated by the
      accumulated e2e and agent-verified workflows; each gap is fixed or
      explicitly re-scoped before the import session runs.
- [ ] The temporary `DIVERGENCES.md` intake is empty after a complete go-live
      audit; canonical specs describe the shipped system exactly.
- [ ] Old tool demoted: no longer the capture target (kept read-only or
      retired — Javier's call, recorded here when made).
- [ ] One week of real use with no data-integrity incident before v1 is
      declared closed.
- [ ] v1 retrospective: remaining wishes filed as post-v1 candidates
      (TUI, recurrence, Today — see `OVERVIEW.md` § "Deferred to v2+"),
      map closed out.

## Standards

CLI-CMD-005, CLI-RELEASE-001/002/003, CLI-DOCS-001/002/003,
CLI-QUALITY-001.
