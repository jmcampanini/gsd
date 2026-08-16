# Milestone 13 — Board view

Data mode: **live**. Depends on: Milestone 11 (boards), Milestone 12
(navigation layer).

Written light; re-review at plan gate.

## Capability

The strategic TUI: one board full-screen, stages as columns, open
projects as cards — the visual answer to "where is everything, and
what deserves to move next?". Read-only by design: this milestone
rides the navigation layer only, and in-TUI stage movement arrives
with Milestone 14's mutation layer (its stage-movement chunk), where
mutations belong. Until then the CLI's `move` is the pen; this view is
the whiteboard.

## Scope

- **A fourth view type** beside root tree, container list, and
  detail: full-screen, one board at a time, columns are the board's
  stages in order, cards are open projects in position order with
  their derived task progress. The no-panes paradigm holds — columns
  are content within one view, not panes.
- **Navigation**: left/right moves across columns, `j`/`k` across
  cards; Enter on a card opens the uniform detail view; Esc returns.
  Entered from the root tree's boards section (activating Milestone
  12's proposed inert rows).
- **Freshness** matches the TUI paradigm: load on entry, re-read on
  re-entry, no polling.
- Column overflow, empty-stage rendering, and whether `/` live filter
  applies inside the board view are settled at plan gate
  (**proposed**: `/` filters cards, columns stay fixed).

## Design language (settled in Milestone 12)

The column view inherits Navigator's design language — the `gsd`
badge and breadcrumb band above, the key-hint band below, Picker
selection, and the record-row vocabulary including the `▥` board
glyph. The settled mock: columns are the board's stages with dim
headings (bold on the selected column); cards are two lines, project
title over dim `done/total` progress; the selected card takes the
accent-edge row fill; `h`/`l` and the reserved `←`/`→` cross
columns while `j`/`k` walk cards. Empty stages render as bare
headings, matching the vertical board view, which remains available;
how the two modes coexist stays this milestone's plan-gate decision.

## Carried from Milestone 12

The Milestone 12 foundation review's fix-now findings were resolved on
the milestone branch. It scheduled no work, so there is no chunk 0. Its
three deferred findings and all still-open carried items follow with
their revisit triggers. Navigator retired the run/unwrap-helper,
tmux-harness, and truncation triggers by firing them; stage history
remains owned by Milestone 18.

- **Date-token assembly consolidation** — the overdue predicate is
  shared (`task.Overdue`), but the `due`/`defer`/`defer→` token
  grammar remains spelled in both `cmd` and the navigator. Revisit
  trigger: the first change to the date-token grammar extracts a
  shared tokens-plus-urgency projection, styling staying
  per-surface.
- **Navigator input-mode key handlers** — filter editing and
  navigation are two inline branches in `updateKey`. Revisit
  trigger: the third input mode (Milestone 15's `:` command line, or
  Milestone 14's confirms if modal) extracts per-mode handlers.
- **Golden frames for navigator pins** — the exact-output pin style
  is deliberate. Revisit trigger: a second broad pin rewrite (chunk
  4 of Milestone 12 was the first) moves full-frame expectations to
  golden files; targeted styled-fragment assertions stay inline.
- **In-flight write lifecycle** — `captureSubmission` (child context,
  cancel-and-wait shutdown, synthesized cancellation for a
  never-scheduled command) is generic to any TUI surface that writes:
  promote it out of capture on the TUI's first writing surface,
  expected at Milestone 14's row verbs.
- **Stage-gate predicate spellings** — the stage-defer gate exists in two
  places, the `available` view in
  `internal/store/migrations/0001_baseline.sql` and the `list --deferred`
  selector in `internal/store/tasks.go`, with a store test pinning their
  agreement. Revisit trigger: the first change to stage-gate semantics
  must touch both sites.
- **Task stage buckets** — Linear-style optional task-to-stage assignment
  stays orthogonal to task status and never gates project movement. Revisit
  when a project's task list grows phases that keep getting encoded in task
  titles.
- **WIP limits** — no per-stage project limit ships today. Revisit when the
  board view makes over-commitment visible and it hurts.
- **Stability-contract prose home** — deferred in Milestone 9: the
  additive-or-full-delete contract's durable prose home once `plans/`
  retires (`AGENTS.md` is the natural candidate); the migration-policy
  and contract lint tests carry the mechanical part regardless.
- **Config report generalization** — on config key #2 (`[serve] addr`,
  arriving with the optional Serve milestone, 16): add source
  classification and tag-derived env/flag spellings to
  go-config-loader's `configreporter` so gsd's renderer becomes a
  generic provenance-row loop, and revisit the reporting/redaction
  contract at the same moment. Keep positional load parameters; do not
  introduce a load-request struct.
- **Genericizing the intentionally-parallel tag service flows** —
  carried from Milestone 6: revisit on the first sibling-divergence bug
  or a post-v1 attach-semantics change.
- **Typed transition spec for `applyTransition`** — board movement and
  promotion do not add an action case to the task store's transition
  switch. Revisit when a new action case enters that switch.
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

## Chunks

1. **Board view scaffold** — column layout, card rendering with
   progress counts, horizontal/vertical navigation, root-tree entry.
2. **Detail integration and polish** — Enter-to-detail round trip,
   empty and overflowing columns, freshness on re-entry.

## User stories

```text
$ gsd tui        # root: …, areas, boards
                 # Enter on "software": the board, column per stage
                 # → to "doing", j to the second card, Enter: detail
                 # Esc: back on the board; Esc: back at the root
```

## Agent-verified end-to-end workflow

Tmux-driven against the real built binary and a seeded temporary
database:

1. The root tree lists boards; Enter opens the board with columns in
   stage order and cards matching `board show --json` for the same
   board.
2. Navigate across columns and cards; Enter opens the correct project
   detail; Esc returns to the same position.
3. Move a project and complete a task via the CLI mid-session;
   re-entering the view reflects the new column and progress count.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md`'s TUI section documents the board view.
