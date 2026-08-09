# Milestone 12 — Navigator

Data mode: **live**. Depends on: Milestone 10 (substrate); the `/` chunk
rides Milestone 8's FTS index.

Written light; re-review at plan gate. The paradigm below is settled;
everything finer is proposed.

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
- **Three view types.**
  - *Root tree*: Inbox, Available, Logbook; then loose projects; then
    areas with their open projects nested. Every container is one jump
    from the root. (**proposed**) A boards section follows the areas,
    one row per board; opening a board waits for Milestone 13's board
    view — until then the rows are inert or absent, settled at plan
    gate.
  - *Container list*: a compact selectable header for the container
    itself above the rows. Enter on the header opens the container's
    detail; Enter on a task opens the task's detail; Enter on a nested
    container drills in. An area lists its open projects, then its
    loose open tasks, composed from existing list operations.
  - *Detail*: one uniform view rendering any entity — area, project, or
    task — mirroring `show` (fields, tags, dates). Notes render as
    plain escaped text; markdown is a parked exploration.
- **Navigation**: `j`/`k`/arrows move (the header is the topmost cursor
  position), Enter descends/opens, Esc/`h` goes back, `q` quits.
  Keyboard-only.
- **Freshness**: data loads when a view is entered; re-entering
  re-reads. No polling or watchers.
- **`/` is incremental search** with the same semantics as `gsd search`
  (in the tree since Milestone 8), filtering the current view live.
  Search implies a query per keystroke, and the virtual index is
  rebuilt per invocation (~25ms at 5,000 documents): decide index
  lifetime — per-call, session-held, or debounced — at this plan gate.

## Chunks

1. **Stack and root tree** — view stack, root tree over the three views
   and the area/project containers.
2. **Container lists** — list views with selectable headers, area
   composition, drill-in/out.
3. **Uniform detail** — the `show`-mirroring detail view for all three
   kinds.
4. **Live filter** — `/` incremental filtering of the current view.

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

## User stories

```text
$ gsd tui        # root: Inbox, Available, Logbook, projects, areas
                 # ↓ into an area: its projects, then loose tasks
                 # ↓ into a project: its tasks
                 # Enter on the header: the project's detail
                 # Esc, Esc, Esc: back at the root; q: gone
```

## Agent-verified end-to-end workflow

Against the real built binary and a seeded temporary database, driving a
tmux session:

1. Root shows the three views and the seeded containers in position
   order; nested projects sit under their areas; loose projects at top
   level.
2. Drill area → project → task detail and back; screens contain the
   expected rows and fields at each step.
3. Header selection opens area and project detail; content matches
   `show --json` for the same IDs.
4. Mutate via the CLI mid-session; re-entering the view reflects it.
5. `/` narrows the current view live; clearing it restores the view.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md` documents `tui` as a shipped command.
