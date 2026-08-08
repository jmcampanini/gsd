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

### Milestone 9 — Go live

Delivered the durable database and closed v1: numbered SQL migrations
embedded in the binary and applied automatically on open, with
`0001_baseline` carrying the accumulated throwaway schema and
`PRAGMA application_id`/`user_version` stamped inside each migration's
transaction (`user_version` 1). The guard ladder refuses foreign
identities, nonempty version-0 files, and databases newer than the
binary with exact `conflict`-coded messages, and a mid-migration
failure rolls back that migration only, resuming on the next open.
`SCHEMA.md`'s additive-or-full-delete stability contract is enforced
by a lint test diffing end states across the migration chain; the
foundation review moved the DDL-only and temporary-schema policies to
the test layer, so a violating migration is a `make check` failure,
not a runtime check. Consolidation reconciled `COMMANDS.md` § Database
to live-era semantics. From here real data enters through daily use
and every schema change ships as a numbered migration.

### Milestone 10 — Capture

Delivered the first TUI surface and the substrate the remaining TUI
milestones stand on: `gsd capture`, a popup-sized single-input Bubble
Tea v2 program built for `tmux display-popup`, writing through
`task.Add` with exactly `gsd add TITLE` semantics — blank Enter is a
no-op, Esc/Ctrl+C cancel at exit 0, a failed write renders its
application error inline and exits 1 on dismiss, and `--json`/non-TTY
invocations are usage errors naming `gsd add` as the noninteractive
path. The substrate: the `internal/tui` package (program construction
from a cobra command, per-program screen mode, the
background-adaptive Catppuccin theme now shared with CLI rendering),
an in-flight-write lifecycle that never closes the database under a
running `Add`, and a tmux-driven e2e harness inside `make check`
(tmux is a documented prerequisite). The foundation review kept the
cursor vanilla Bubble Tea (upstream `Cursor()`, probe deleted),
converted the model to a phase enum, split the terminal-detection
seam into typed reader/writer dependencies, and consolidated
control-character escaping into `internal/text`; the substrate
promotion triggers carry in `MILESTONE_12.md`.

## Active roadmap

Active planning begins with the Boards milestone:

| # | Milestone | Capability delivered | Data mode |
|---|-----------|----------------------|-----------|
| 11 | [Boards](MILESTONE_11.md) | Boards, stages, and stage-aware tasks in the CLI | **live** |
| 12 | [Navigator](MILESTONE_12.md) | Read-only full-screen `gsd tui` | **live** |
| 13 | [Board view](MILESTONE_13.md) | Read-only strategic board in the TUI | **live** |
| 14 | [Row verbs](MILESTONE_14.md) | Single-key mutations and reorder in the TUI | **live** |
| 15 | [Input grammar](MILESTONE_15.md) | `:` command line and richer capture | **live** |

v1 closed when Go live landed; Capture shipped as the first post-v1
milestone.

Boards (11) is the strategic layer of the data model — pipelines
projects move through — and lands CLI-first, independent of the TUI
track.

The TUI shipped its substrate as Milestone 10 and continues as
Milestones 12–15, sequenced by
foundational layer rather than feature count: each milestone boundary
marks a foundation review that later TUI work builds on — substrate,
then navigation, then mutation, then grammar. Leaf features ride their
layer's milestone as chunks: live `/` search in Navigator,
grab-and-move reorder and single-key stage movement in Row verbs,
capture runner mode in Input grammar. Board view (13) is a feature
milestone riding the layers rather than a layer itself: read-only on
the navigation layer, with its mutation arriving via Row verbs. Pane
and split layouts, mouse support, and markdown note rendering are
parked explorations, deliberately unnumbered.

### Optional milestones

Sequenced after the TUI, in live data mode, and entering planning only
when their trigger fires:

| # | Milestone | Capability delivered | Activation trigger |
|---|-----------|----------------------|--------------------|
| 16 | [Serve](MILESTONE_16.md) | Loopback HTTP API | A consumer that can't exec the CLI (browser frontend, remote agent) |
| 17 | [Query](MILESTONE_17.md) | Read-only SQL escape hatch | Daily use demonstrates the need for raw SQL |
| 18 | [History](MILESTONE_18.md) | Change tracking over time | A question about the past that current state and the logbook cannot answer |

## Data policy

- **Before Go live (throwaway era, historical):** development ran
  without migrations. Each schema-changing milestone bumped
  `PRAGMA user_version` in a dev-only range
  (`9000 + roadmap milestone number`) so throwaway stamps could never
  collide with real migration numbers, and the binary failed loud on a
  mismatch; development databases were disposable by declaration. No
  dev-range guard survives Go live — a leftover throwaway database
  never carried gsd's `application_id`, so it fails the
  foreign-identity refusal in `COMMANDS.md` § Database.
- **Go live onward (live):** the accumulated schema became migration
  `0001_baseline`; every later schema change ships as a numbered
  migration that applies automatically on open. Real data must survive
  every subsequent milestone under the schema stability contract in
  `SCHEMA.md`.

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
  loopback-only, no-auth HTTP API with full CLI parity minus `config`. Grammar: `/v1` resources, action endpoints for lifecycle
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
- 2026-08-06, Milestone 9 cancellation: the Query milestone (`gsd query`,
  read-only SQL with the schema as public contract) was cancelled at its
  plan gate, before implementation; raw SQL access returns only if daily
  use demonstrates the need. The schema-as-public-API framing left the
  canonical specs with it — `SCHEMA.md`'s contract section now states
  the stability rules Go live's migrations require. Milestone numbering
  is retained; the roadmap resumes at 10. Milestone 8's carried deferred
  items and the functional completeness audit moved into the Capture
  milestone plan (its surviving deferrals now carry in
  `MILESTONE_11.md`). Go live's install story partially shipped early the
  same day: `Formula/gsd.rb` and the self-tap README install
  instructions landed via PR #53.
- 2026-08-06, roadmap restructure and renumbering: Serve moved behind
  the TUI and became optional — nothing needs HTTP today (the TUI is
  in-process and agents drive the CLI's `--json` surface), and
  deferring it reaches daily use sooner; it activates only when a
  consumer that can't exec the CLI appears (browser frontend, remote
  agent). Query returned to the roadmap as an optional milestone after
  Serve, keeping its cancellation trigger. The roadmap was renumbered
  as a clean break — Go live retakes 9, the TUI is 10–13, Serve is 14,
  Query is 15; earlier decision entries keep the numbers that were
  current when they were written. Go live itself was cut to the
  migration runner: the install story had already shipped (PR #53 and
  a verified `brew install --HEAD` on the target machine; the README
  landing-page rewrite and completion docs were deliberately dropped),
  and the agent-driven import and functional completeness audit were
  descoped — real data enters through daily use from a fresh start,
  and real use surfaces gaps over time, so v1 closes when Go live
  lands, with no import, soak, or old-tool-demotion gate.
- 2026-08-07, Boards planning interview: boards join the roadmap as
  Milestone 11 — the strategic lens over projects. A board is a
  user-defined pipeline of named, ordered stages; a project subscribes
  to at most one board and occupies exactly one stage, stored on the
  project with the board rendered as a grouped lens (prior art:
  Linear, GitHub Projects, and Notion all store stage and render
  columns; Trello's column-as-storage model is the counterexample).
  Boards are global like tags; membership is optional; stage is
  orthogonal to `open`/`done`/`cancelled` — the board shows open
  projects only, the last stage never means done, and the logbook
  keeps owning finished work. Tasks stay tactical with exactly one
  narrow stage relationship: `defer until stage` (hidden from
  `available` until the project reaches or passes the named stage,
  composing with date defer) plus an opt-in promotes marker
  (completing the task transactionally moves the project one stage
  forward — one-way, relative, a reported no-op at the last stage).
  Derived-from-tasks advancement and hard stage-gating were rejected;
  Linear-milestone-style stage buckets and WIP limits were deferred
  with recorded triggers. Vocabulary fixed: board, stage, column
  (presentation only), move (the verb), promote. The schema folds
  into `0001_baseline` as a pre-users clean break — no `0002`
  migration; the existing personal database is recreated once. The
  roadmap renumbered as a clean break, per the 2026-08-06 precedent:
  Boards takes 11, Navigator 12, the new read-only Board view 13
  (its stage-movement key rides Row verbs), Row verbs 14, Input
  grammar 15, Serve 16, Query 17, and History enters as optional 18 —
  deliberately underspecified change-over-time tracking, recording
  from activation, not retroactively. Earlier decision entries keep
  the numbers that were current when they were written.
