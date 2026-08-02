# Milestone Plan (v1)

gsd is built as vertical slices. Every active milestone ends with working,
demonstrable capability, never a horizontal layer. The canonical product
specification lives in `OVERVIEW.md`, `COMMANDS.md`, and `SCHEMA.md`. An active
milestone file defines the authoritative outcomes and acceptance boundary; any
implementation details or chunks it contains are suggestive. Root `PLAN.md` is
the authoritative implementation and verification plan for the active
milestone.

[`PROCESS.md`](PROCESS.md) is the canonical contract for planning, branch
ancestry, chunk design, pull requests, review and verification gates,
consolidation, plan retirement, and landing a milestone in `main`.

## Completed

These milestones are consolidated and their temporary execution plans have
been permanently retired under [`PROCESS.md`](PROCESS.md). Current code,
tests, and canonical specifications are authoritative for their completed
behavior.

### Milestone 0 — Scaffolding

Established the standards-compliant repository, command-tree and version
spine, complete local `make check` contract, CI, and initial codified
engineering guardrails.

### Milestone 1 — Capture loop

Delivered the complete bare-inbox task lifecycle: add, list, show, edit,
complete, cancel, reopen, and delete, with persistent SQLite state, human and
JSON output, stable application errors, and end-to-end coverage.

### Milestone 2 — Time

Delivered strict local-calendar due and defer dates, date-aware list filters,
and the actionable `available` view with human and JSON output and end-to-end
coverage.

### Milestone 3 — Projects

Delivered projects with task containment and append-on-re-parent, the
transactional completion/cancellation cascade with narration, the
resolved-project guard, RESTRICT-guarded deletion with an explicit
`--recursive` opt-in, and the interleaved `logbook`.

### Milestone 4 — Areas

Delivered areas holding projects and loose tasks with schema-enforced
containment exclusivity, governing-area inheritance through `available`
and the enriched views, archive/unarchive as retired history with typed
unarchive-first guidance, and RESTRICT-guarded area deletion with a
transactional `--recursive` opt-in reporting a container-grouped
envelope.

### Milestone 5 — Tags

Delivered the flat case-insensitive tag namespace across tasks,
projects, and areas: name-addressed administration with usage counts,
attach/detach verbs and `--tag` creation and list filtering on all
three nouns, idempotent attachment, cascade detachment on deletion, and
`tags` arrays completing every entity JSON envelope — converging the
schema byte-for-byte with `SCHEMA.md`. Consolidation also landed the
store transaction seam across all stores, read-then-validate guard
classification, and the shared `internal/domain` validation package.

## Active roadmap

Active planning begins with the Config milestone:

| # | Milestone | Capability delivered | Data mode |
|---|-----------|----------------------|-----------|
| 6 | [Config](MILESTONE_6.md) | Config file via go-config-loader; color modes | throwaway |
| 7 | [Reorder](MILESTONE_7.md) | Manual ordering | throwaway |
| 8 | [Search](MILESTONE_8.md) | FTS5 `search` | throwaway |
| 9 | [Query](MILESTONE_9.md) | Read-only SQL; schema as public contract | throwaway |
| 10 | [Go live](MILESTONE_10.md) | Migrations, install story, real-data import | **live** |

The TUI is deliberately not in the v1 map. It remains a separate post-v1
effort described as a forward-looking target in `COMMANDS.md` and
`OVERVIEW.md`.

## Data policy

- **Before Go live (throwaway):** no migrations. Each schema-changing
  milestone bumps `PRAGMA user_version`, stamped in a dev-only range
  (`9000 + roadmap milestone number`) so throwaway stamps can never collide
  with real migration numbers later. On mismatch, the binary fails loud with
  "throwaway db from an older milestone — delete it." Development databases
  are disposable by declaration.
- **Go live onward (live):** the accumulated schema becomes migration
  `0001_baseline`; every later schema change ships as a numbered migration.
  Real data must survive every subsequent milestone under the schema stability
  contract in `SCHEMA.md`.

## Decisions and history

- Stack: Go (current release), Cobra, charm.land v2 packages, per
  jmcampanini/cli-standards. Fleet patterns come from cmdk/overlay.
- SQLite driver: `modernc.org/sqlite` (pure Go, no cgo; FTS5 supported).
- `--json` plus structured application errors is a baseline property of every
  behavioral command. Usage errors remain human-readable with exit code 2, as
  specified in `COMMANDS.md`.
- Schema is created incrementally per milestone rather than all up front.
- Real data enters only during Go live, via an agent driving the CLI against an
  export from the current tool; no import code is added to gsd.
- The canonical v1 config surface is minimal: `db_path` and `color` only. Its
  contract is in `COMMANDS.md`, with implementation scheduled for Config.
- Initial stack, data-policy, TUI-scope, error-rendering, and config decisions
  were settled during planning and implementation interviews on 2026-07-26
  and 2026-07-27, then reconciled into the canonical specifications.
- 2026-08-02, Milestone 5 foundation review: Go live moved to the end of
  the roadmap (Config → Reorder → Search → Query → Go live) so the
  complete written-down v1 surface is demonstrated on throwaway data
  before real data enters. The go-live baseline therefore includes the
  FTS schema, and v1 ships no live migrations.
- Same review: entity `tags` arrays become alphabetical (`NOCASE`),
  matching `tags list` — specified in the canonical docs now, shipped by
  Milestone 6 chunk 0, which also consolidates the review's structural
  findings. The amended transaction-authority rule in `AGENTS.md` was
  ratified: stores own implementation-detail atomicity, services own
  use-case composition.
