# Milestone 12 — Navigator

Data mode: **live**. Depends on: Milestone 10 (substrate) and
Milestone 11 (boards, for the boards lens).

Settled at the 2026-08-08 plan gate; root `PLAN.md` records the full
design. The scope below was reconciled to match.

## Capability

`gsd tui` — the full TUI experience, read-only: navigate the whole
system from a root tree down to any entity's detail without mutating
anything. This milestone consolidates the navigation layer — the view
stack, view types, selection model, and data binding — that every later
TUI verb, reorder, and command-line milestone acts on.

## Scope

- **Full-screen views, no panes.** Exactly one view on screen;
  navigating replaces the screen entirely. The same structure works in
  a full terminal and a tmux popup. Pane and split layouts are parked
  explorations, deliberately out of scope.
- **Two lenses, one tree.** Boards and areas are two lenses over the
  same objects — projects; neither contains the other. The levels are
  the root, the lenses, projects, tasks. The root shows Inbox,
  Available, Logbook, then Boards and Areas.
  - *Collections* (Boards, Areas): lists of entities with no header —
    a collection is not itself an entity. Board rows carry the stage
    chain; the areas collection ends with a `(no area)` pseudo-row
    holding loose projects behind a plain non-selectable title.
  - *Containers* (a board, an area, a project): a compact selectable
    header for the container itself above the rows. Enter/`l` opens
    the selected header's detail or opens/drills the selected row. An
    area lists its open projects, then its loose open
    tasks; a board lists its open projects grouped by stage with
    progress counts, composed from existing operations. The board
    list is not throwaway — Milestone 13's column view arrives beside
    it.
  - *Detail*: one uniform view rendering any entity — task, project,
    area, or board — mirroring `show` (fields, tags, dates). Notes
    render as plain escaped text; markdown is a parked exploration.
- **Navigation**: `j`/`k`/arrows move (the header is the topmost cursor
  position), Enter/`l` descends/opens, Esc/`h` goes back; at the root,
  Esc quits and `h` is inert. `q` and Ctrl+C quit from anywhere. Keyboard-only.
- **Freshness**: data loads when a view is entered; re-entering
  re-reads. No polling or watchers.
- **`/` is an incremental fuzzy filter** — snacks-picker semantics,
  not FTS: in-memory fuzzy subsequence matching with smart-case over
  the current view's visible rows, matched characters highlighted,
  view structure preserved. View-local; a global picker is parked
  (revisit at Milestone 15's command line). `gsd search` remains the
  CLI's FTS surface; D-004 tracks the `COMMANDS.md` TUI-section
  mismatch until consolidation.

## Chunks

1. **The skeleton stands** — view stack, root view, the three task
   views, and the two collection views.
2. **Every hall opens** — the container views: board (stage-grouped),
   area, project, and `(no area)`.
3. **Every door opens** — the uniform `show`-mirroring detail view for
   all four kinds.
4. **Type to find** — `/` fuzzy filtering of the current view.

## Carried from Milestone 11

The Milestone 11 foundation review's fix-now findings were resolved on the
milestone branch. It scheduled no work, so there is no chunk 0. Its one
deliberately deferred finding and all deferred items due in Navigator carry
forward below with their revisit triggers. Stage history remains owned by
Milestone 18. The Milestone 10 substrate triggers remain because Navigator is
their expected next consumer.

- **In-flight write lifecycle** — `captureSubmission` (child context,
  cancel-and-wait shutdown, synthesized cancellation for a
  never-scheduled command) is generic to any TUI surface that writes:
  promote it out of capture on Navigator's first writing surface.
- **Program run/unwrap helper** — `RunCapture` hand-unwraps the
  `programModel` wrapper to recover its final model; extract a shared
  runner when the second surface repeats the boilerplate.
- **tmux e2e harness** — the private-server, `send-keys`, and
  status-file helpers live in `e2e/capture_test.go` under capture
  names: promote them to shared e2e helpers on the second tmux-driven
  test.
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
- **Truncation idiom consolidation** — `ansi.Truncate` with an `…`
  tail has one production call site (the capture error footer). On the
  second truncate-with-ellipsis call site, promote an `Ellipsize` helper
  into `internal/text` beside the escaping helper. Milestone 11 did not
  trip the trigger because `board show` does not truncate.
- **Stability-contract prose home** — deferred in Milestone 9: the
  additive-or-full-delete contract's durable prose home once `plans/`
  retires (`AGENTS.md` is the natural candidate); the migration-policy
  and contract lint tests carry the mechanical part regardless.
- **Config report generalization** — on config key #2 (`[serve] addr`,
  arriving with the optional Serve milestone, 16): add source
  classification and tag-derived env/flag spellings to
  go-config-loader's `configreporter` so gsd's renderer becomes a
  generic provenance-row loop like the sibling CLIs, and revisit the
  reporting/redaction contract at the same moment. No load-request
  struct — positional load parameters are the family idiom.
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

## User stories

```text
$ gsd tui        # root: Inbox, Available, Logbook, Boards, Areas
                 # ↓ into Areas: the areas, then (no area)
                 # ↓ into an area: its projects, then loose tasks
                 # ↓ into a project: its tasks; Enter: a task's detail
                 # Enter on the header: the project's detail
                 # ↓ into Boards → a board: projects grouped by stage
                 # /: fuzzy-narrow the view; Esc: clear, then back
                 # Esc, Esc, Esc: back at the root; q: gone
```

## Agent-verified end-to-end workflow

Against the real built binary and a seeded temporary database, driving a
tmux session:

1. The root shows Inbox, Available, Logbook, Boards, Areas; the
   collections list the seeded boards (with stage chains) and active
   areas in position order, with `(no area)` last.
2. Drill Areas → area → project → task detail and back; screens
   contain the expected rows and fields at each step.
3. Header selection opens area, project, and board detail; content
   matches `show --json` for the same identities.
4. Mutate via the CLI mid-session; re-entering the view reflects it.
5. `/` fuzzy-narrows the current view live; clearing it restores the
   view.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md` documents `tui` as a shipped command.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003; TUI-applicable standards re-checked
at plan gate.
