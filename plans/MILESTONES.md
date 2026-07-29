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

## Active roadmap

Active planning begins with the Time milestone:

| # | Milestone | Capability delivered | Data mode |
|---|-----------|----------------------|-----------|
| 2 | [Time](MILESTONE_2.md) | Defer/due dates; `available` | throwaway |
| 3 | [Projects](MILESTONE_3.md) | Projects, cascade, `logbook` | throwaway |
| 4 | [Areas](MILESTONE_4.md) | Areas, archive, governing-area inheritance | throwaway |
| 5 | [Tags](MILESTONE_5.md) | Tags across all entities | throwaway |
| 6 | [Config](MILESTONE_6.md) | Config file via go-config-loader; color modes | throwaway |
| 7 | [Go live](MILESTONE_7.md) | Migrations, install story, real-data import | **live** |
| 8 | [Reorder](MILESTONE_8.md) | Manual ordering | live |
| 9 | [Search](MILESTONE_9.md) | FTS5 `search` | live |
| 10 | [Query](MILESTONE_10.md) | Read-only SQL; schema as public contract | live |

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
