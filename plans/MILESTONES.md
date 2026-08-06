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

### Milestone 6 — Config

Delivered the canonical v1 config contract: a discovered or explicit
TOML file loaded through go-config-loader with defaults → file → env →
flag precedence and fail-loud invalid-file handling, db-path resolution
moved onto the loaded config, and the TOML-only `gsd config` report
with normalized `--provenance` source comments. Landed the settled
visual system — standards-pure per-stream color modes,
background-adaptive Catppuccin accents, quiet-headed collection tables,
glyph-led records and mutation lines, and `├`/`└` cascade trees — while
JSON envelopes stayed byte-identical to Milestone 5. Consolidation also
landed the root/core store split, read-then-delete cascades,
service-owned read-path tag resolution, `domain.TagNames`, and
alphabetical tag arrays (`user_version` 9006).

### Milestone 7 — Reorder

Delivered manual ordering — `reorder` on tasks, projects, and areas
with `--first`/`--last`/`--after`/`--before` placements: sibling-relative
container semantics rejecting cross-container and self-references,
status-blind contiguous renumbering from 0 with `updated_at` bumped on
the moved row only, no-op placements succeeding as ordinary reorders,
bare-entity JSON echoes, and `~ Reordered:` mutation lines — no schema
change (`user_version` stays 9006). Chunk 0 applied the Milestone 6
review's scheduled finding, moving filtered-list area validation for
projects into the service; consolidation codified the
services-own-validation store contract in `AGENTS.md` and ordinal-only
position semantics in `SCHEMA.md`.

### Milestone 8 — Search

Delivered full-text search — `gsd search "EXPR"` passing FTS5 match
syntax through to the title, tags, and note of every task, project,
and area across all statuses and archived areas, with `--related`
widening the same expression through inherited container titles and
tags: direct matches tiered above context-only matches, weighted-bm25
relevance with kind/id tie-breaks, kind-discriminated complete entity
rows in JSON, and a kind/id/title/status/context human table. The
index is virtual — a temp FTS5 table built per invocation from live
data — so nothing persists, results are never stale, and
`user_version` stays 9006. Consolidation reconciled the search grammar
in `COMMANDS.md` (filter flags descoped in favor of future
in-expression operators) and the now-vacuous no-live-migrations note;
wrap-up aligned `TagNames` JSON with the shared non-HTML-escaped
output policy across all tag-emitting commands.

## Active roadmap

Active planning begins with the Query milestone:

| # | Milestone | Capability delivered | Data mode |
|---|-----------|----------------------|-----------|
| 9 | [Query](MILESTONE_9.md) | Read-only SQL; schema as public contract | throwaway |
| 10 | [Go live](MILESTONE_10.md) | Migrations, install story, real-data import | **live** |
| 11 | [Serve](MILESTONE_11.md) | Loopback HTTP API | **live** |
| 12 | [Capture](MILESTONE_12.md) | TUI substrate + `gsd capture` popup | **live** |
| 13 | [Navigator](MILESTONE_13.md) | Read-only full-screen `gsd tui` | **live** |
| 14 | [Row verbs](MILESTONE_14.md) | Single-key mutations and reorder in the TUI | **live** |
| 15 | [Input grammar](MILESTONE_15.md) | `:` command line and richer capture | **live** |

v1 closes with Go live; Serve is the first post-v1 milestone.

The TUI enters the map as Milestones 12–15, sequenced by foundational
layer rather than feature count: each milestone boundary marks a
foundation review that later TUI work builds on — substrate, then
navigation, then mutation, then grammar. Leaf features ride their
layer's milestone as chunks: live `/` search in Navigator, grab-and-move
reorder in Row verbs, capture runner mode in Input grammar. Pane and
split layouts, mouse support, and markdown note rendering are parked
explorations, deliberately unnumbered.

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
- The canonical v1 config surface is minimal: `db_path` only. Its contract
  is in `COMMANDS.md`, with implementation scheduled for Config. Color is
  flag and environment behavior (CLI-OUTPUT-001), never a config key.
- Initial stack, data-policy, TUI-scope, error-rendering, and config decisions
  were settled during planning and implementation interviews on 2026-07-26
  and 2026-07-27, then reconciled into the canonical specifications.
- 2026-08-02, Milestone 5 foundation review: Go live moved to the end of
  the roadmap (Config → Reorder → Search → Query → Go live) so the
  complete written-down v1 surface is demonstrated on throwaway data
  before real data enters. The search index is virtual — built per
  invocation, nothing persists — so v1 ships no live migrations, now
  vacuously.
- Same review: entity `tags` arrays become alphabetical (`NOCASE`),
  matching `tags list` — specified in the canonical docs now, shipped by
  Milestone 6 chunk 0, which also consolidates the review's structural
  findings. The amended transaction-authority rule in `AGENTS.md` was
  ratified: stores own implementation-detail atomicity, services own
  use-case composition.
- 2026-08-02, Milestone 6 planning: color was removed from the config
  surface — the chain is strictly CLI-OUTPUT-001 (`--color` > nonempty
  `NO_COLOR` > per-stream auto), with no `GSD_COLOR` and no TOML key. The
  human-output styling system was settled by three blind pairwise test
  rounds plus a glyph tournament: quiet-chrome collection tables, glyph-led
  records and mutation lines, standard `├`/`└` cascade trees, and red/green
  state accents drawn from Catppuccin Latte/Frappé selected by terminal
  background, accents-only.
- 2026-08-04, Serve planning interview: Milestone 11 adds `gsd serve`, a
  loopback-only, no-auth HTTP API with full CLI parity minus `query` and
  `config`. Grammar: `/v1` resources, action endpoints for lifecycle
  transitions (cascades return the CLI envelopes), merge-patch `PATCH`
  with `null`-clears for edits; wire JSON and the error envelope are the
  CLI `--json` shapes unchanged, mapped 404/400/409/500. The bind address
  is config key #2 (`[serve] addr`, `GSD_SERVE_ADDR`, `--addr`, default
  `127.0.0.1:8473` — "473" is gsd on a keypad), tripping Milestone 7's
  deferred config-report generalization. The canonical contract is a new
  `plans/API.md`; no OpenAPI in v1. Concurrency is documented, not
  enforced (last-write-wins `PATCH`, non-idempotent creates); the
  schema-skew guard and browser-borne hardening are deferred with
  recorded revisit triggers in `MILESTONE_11.md`.
- 2026-08-04, TUI planning interview: the TUI joins the roadmap as
  Milestones 12–15 (Capture, Navigator, Row verbs, Input grammar),
  sequenced by reviewable foundational layers. The settled paradigm is
  full-screen views with no panes: a root tree (Inbox, Available,
  Logbook, loose projects, areas with open projects nested), container
  lists with a compact selectable header, and a uniform detail view
  mirroring `show`; keyboard-only, load-on-entry freshness. `gsd
  capture` is a tmux-popup-sized pure-capture surface (title → inbox)
  and lands first as the substrate proof. The `COMMANDS.md` TUI section
  was reconciled from the earlier three-pane sketch to this paradigm;
  panes, mouse, and markdown notes are parked explorations.
