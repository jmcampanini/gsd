# Milestone 12 — Navigator: root implementation plan

[`plans/MILESTONE_12.md`](plans/MILESTONE_12.md) is authoritative for
this milestone's outcomes and acceptance boundary;
[`plans/PROCESS.md`](plans/PROCESS.md) governs workflow, review, and
consolidation. This plan is temporary and is retired at consolidation.

## Progress

- [x] Chunk 1 — The skeleton stands
- [x] Chunk 2 — Every hall opens
- [x] Chunk 3 — Every door opens
- [x] Chunk 4 — The lights come on
- [ ] Chunk 5 — Type to find

There is no chunk 0: the Milestone 11 foundation review scheduled
nothing; the deferred items carry forward in `MILESTONE_12.md` with
their revisit triggers.

## Settled design

Plan-gate interview, 2026-08-08. The governing decision: **boards and
areas are two lenses over the same objects — projects.** Neither
contains the other, so the tree is uniform — root, then the lenses,
then projects, then tasks — and every view is one of three shapes.
`MILESTONE_12.md`'s scope, chunks, and user stories were reconciled to
match; the `COMMANDS.md` TUI-section mismatch is recorded as D-004
until consolidation.

- **The tree.** The root shows Inbox, Available, Logbook, then Boards
  and Areas. Boards and Areas are *collections* — lists of entities
  with no header, since a collection is not itself an entity. A board,
  an area, and a project are *containers* — a compact selectable
  header above the rows, the header being the topmost cursor position;
  Enter or `l` on the header opens the container's detail. The areas
  collection ends with a `(no area)` pseudo-row holding loose
  projects; its list carries a plain non-selectable title because
  there is no entity behind it. Tasks are leaves.
- **Board container view.** A board opens as a stage-grouped list
  composed from `board.ShowByID`: stage headings in position order, open
  projects in stage position order with derived done/total progress.
  This rendering is not throwaway — Milestone 13's column view
  arrives beside it, and how the two coexist is that milestone's
  plan-gate decision.
- **Uniform detail.** One view renders any entity — task, project,
  area, or board — mirroring `show`'s field order: tasks (project,
  area, note, due on, defer until, defer stage, promotes, exit
  stamps, status, position, timestamps, tags), projects (adding the
  board/stage location row), areas (note, archived at, position,
  timestamps, tags), boards (note, position, stages in order,
  timestamps; boards carry no tags). Notes render as plain escaped
  text with line feeds preserved; markdown stays a parked exploration.
- **Content defaults mirror the CLI.** Active areas only; open
  projects and tasks in containers — the logbook is where resolved
  work lives; board rows in the collection render `NAME stage → …`
  like `boards list`; task rows mirror the CLI's list columns, so no
  fourth container-title projection appears. No count badges. Rows
  truncate to the terminal width with `…` and never wrap. Long views
  stay within the terminal height and follow the selected row.
- **Navigation.** `j`/`k`/arrows move; Enter/`l` descends into a
  container or opens a detail; Esc/`h` goes back; Esc clears an
  active filter before it navigates; Esc at the root quits, while `q`
  and Ctrl+C quit from anywhere. `←`/`→` stay unbound, reserved for
  Milestone 13's columns. The view stack keeps
  one cursor per view, restored by entity identity when returning and
  clamped when the row has vanished.
- **Freshness.** A view loads its data when entered and re-reads when
  re-entered, including on pop-return. Entity views reload every field
  by stable ID. No polling, no watchers.
- **Errors.** A failure before the program starts exits 1 through the
  standard stderr path. An in-session view load failure renders its
  application error inline behind the red accent with navigation
  alive; Esc backs out of the failed view.
- **`/` is a fuzzy filter, not FTS.** snacks-picker semantics:
  in-memory fuzzy subsequence matching over the current view's
  visible row text, smart-case (an all-lowercase pattern matches
  case-insensitively; any uppercase requires a case match), with
  matched characters highlighted. `sahilm/fuzzy` — already in the
  module graph via bubbles — becomes a direct dependency. The view
  keeps its structure: sections and headers stay, non-matching rows
  hide, the cursor clamps to the matched set; a blank pattern means
  no filter; the filter is per-view and drops on any navigation.
  List-shaped views filter; detail does not. The filter is
  view-local: a global picker is parked, revisited at Milestone 15's
  command line. The FTS index goes untouched by the TUI — the
  milestone's index-lifetime question is moot, `gsd search` remains
  the CLI's FTS surface, and the `search.Hit`
  constructors/accessors deferral stays parked with its trigger.
- **Command contract.** `gsd tui` takes no arguments; `--json` and
  non-TTY invocation (checked per stream) are usage errors (exit 2)
  whose messages name the CLI as the noninteractive path. The full
  configuration chain applies; the database opens only when the
  program starts, through the injected factory, so help and parse
  failures never touch it. Color follows the existing chain resolved
  in cmd (`--color` > nonempty `NO_COLOR` > per-stream auto);
  light/dark is detected in-program; the program runs in alt-screen.
- **Architecture.** A new `internal/tui/navigator` package owns the
  view stack, view models, rendering, and the fuzzy matcher; the
  substrate — program construction, theme, and the newly shared
  runner — stays in `internal/tui`, and capture stays put this
  milestone. Navigator declares an exported dependency struct over
  the narrow service interfaces it reads (tasks, projects, areas,
  boards, logbook); cmd assembles it inside `withApplications`, and
  the innermost root constructor generalizes its capture-runner seam
  into a runners seam. All reads reach stores through services.
- **Carried-deferral dispositions.** Fires now: the shared program
  run/unwrap helper (second surface — chunk 1), the tmux e2e helper
  promotion (second tmux-driven test — chunk 1), and the `Ellipsize`
  promotion into `internal/text` (second truncate-with-ellipsis call
  site — chunk 2, migrating capture's footer call site). Does not
  fire: the in-flight write lifecycle (Navigator never writes) and
  the fourth container-title projection (rows stay on CLI list
  columns) — both carry forward with their triggers. Moot: the
  search index lifetime.
- **Standards, re-checked at this gate.** CLI-CMD-002: `RunE` with
  coded errors to the boundary. CLI-CMD-003: the TUI owns stdout in
  alt-screen; diagnostics stay on stderr. CLI-OUTPUT-001 governs the
  color chain; CLI-OUTPUT-003 is vacuous (`--json` is refused);
  CLI-OUTPUT-004 is the per-stream interactive guard with actionable
  refusal. tmux remains the exercised development and CI
  prerequisite.

## Chunk 1 — The skeleton stands

Human outcome: `gsd tui` opens to the root; `j`/`k`/arrows walk
Inbox, Available, Logbook, Boards, and Areas; Enter/`l` opens the three
task views and the two collections; Esc returns; `q` (or Esc at the
root) quits.

Implementation:

- [x] `internal/tui`: extract the shared program runner — run the
      program and unwrap the final model behind one helper — and move
      capture onto it; capture behavior unchanged.
- [x] `internal/tui/navigator`: view stack (push/pop, one cursor per
      view restored by entity identity with clamping), the exported
      dependency struct, view-entry loading with re-read on re-entry,
      and the inline error state behind the red accent.
- [x] `internal/tui/navigator`: root view (five fixed rows);
      Inbox/Available/Logbook lists over `Inbox`, `Available`, and
      `logbook.List` with rows mirroring the CLI columns; boards
      collection (`NAME stage → …` from `board.List`) and areas
      collection (active areas from `area.List`, then the `(no area)`
      pseudo-row). Enter/`l` on rows inside these views waits for chunks
      2–3.
- [x] `cmd/tui.go` and root wiring: `gsd tui` registered; guards
      before the factory (no arguments, `--json` refused, per-stream
      TTY checks) naming the CLI path; color resolved in cmd;
      alt-screen program through the shared runner; the innermost
      constructor's capture-runner seam generalized to a runners
      seam.
- [x] `e2e`: promote the capture tmux helpers (private server,
      `send-keys`, status file, pane polling) to shared harness
      helpers; add a navigator smoke test — seeded database, root
      renders all five rows in a full-size pane, `q` exits 0.

Verification (primary owners: navigator model tests with fake
dependencies; cmd tests for guards and wiring; e2e for real-terminal
smoke):

- [x] Model: five root rows in order; movement clamps at both ends;
      Enter/`l` pushes each of the five views; Esc/`h` pops; Esc at root and
      `q` quit; re-entering a view re-calls its loader; a load
      failure renders inline and Esc backs out.
- [x] Model: boards rows show the stage chain in position order;
      areas rows are active-only in position order with `(no area)`
      last; task and logbook rows mirror the CLI columns.
- [x] Shared runner: the capture suite stays green on the extracted
      helper; the runner returns the final model and propagates
      program errors.
- [x] cmd: `--json`, non-TTY stdin, non-TTY stdout, and positional
      arguments are usage errors (exit 2) naming the CLI; the factory
      opens once at run and never for `--help`.
- [x] e2e: the smoke test passes on the promoted helpers; the capture
      e2e stays green on the shared harness.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/12-chunk-1.html`; pane frames
captured per the capture precedent), exact commands:

```sh
gsd --db .sandbox/demo12.db areas add "Home"
gsd --db .sandbox/demo12.db areas add "Work"
gsd --db .sandbox/demo12.db boards add software --stage research \
    --stage doing --stage review
gsd --db .sandbox/demo12.db add "Call plumber"
gsd --db .sandbox/demo12.db tui
    # j/k across the root; Enter on Inbox; Esc; Enter on Boards;
    # Esc; Enter on Areas; Esc; q
```

- [x] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database,
      capture the frames into the deck, and pass local `make check`.

## Chunk 2 — Every hall opens

Human outcome: drill root → Areas → an area → a project and see its
tasks; root → Boards → a board and see its projects grouped by stage
with progress counts; `(no area)` lists the loose projects — the same
objects through two lenses.

Implementation:

- [x] `internal/text`: promote `Ellipsize` (truncate with `…`) beside
      the escaping helper; migrate capture's footer call site;
      navigator rows truncate through it.
- [x] `internal/tui/navigator` vertical viewport: bound rendered views
      to terminal height and keep the selected row visible across
      movement and resize.
- [x] `internal/tui/navigator` container views: area (selectable
      header; open projects, then loose open tasks under section
      headings), project (header; open tasks), board (header; stage
      headings in position order with open projects and done/total
      progress from `board.ShowByID`), and the loose-projects list under
      a plain non-selectable `(no area)` title.
- [x] Data composition: `project.List` (open, by area; `AreaID == nil`
      filtered client-side for loose), `task.List` (open, by project
      and by area), `board.ShowByID`. Headers sit as the topmost cursor
      position; Enter/`l` on them activates in chunk 3.

Verification (primary owners: navigator model tests; text tests for
`Ellipsize`):

- [x] Model: area composition order (projects, then loose tasks) with
      section headings; board grouping and ordering match
      `board.ShowByID` with progress rendered; project tasks in
      `position, id` order; `(no area)` holds exactly the area-less
      projects, boarded or not.
- [x] Model: the header is the topmost cursor position; drill in and
      out from the collections; the cursor restores by identity after
      pop and clamps when the row is gone.
- [x] Model: area, project, and board containers reload by stable ID
      and render renamed headers on pop-return.
- [x] Model: root and structured container viewports remain height
      bounded and keep selections visible after movement and resize.
- [x] Board service/store: `ShowByID` validates and assembles the same
      grouped projection through a real ID lookup.
- [x] text: `Ellipsize` width and ellipsis semantics; capture's
      footer rendering unchanged at its migrated call site.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/12-chunk-2.html`), exact
commands:

```sh
gsd --db .sandbox/demo12.db areas add "Home"
gsd --db .sandbox/demo12.db areas add "Work"
gsd --db .sandbox/demo12.db boards add software --stage research \
    --stage doing --stage review
gsd --db .sandbox/demo12.db projects add "Kitchen reno" --area 1
gsd --db .sandbox/demo12.db projects add "Blog rewrite"
gsd --db .sandbox/demo12.db projects add "gsd milestone 12" \
    --board software
gsd --db .sandbox/demo12.db project move 3 doing
gsd --db .sandbox/demo12.db add "Buy cabinet pulls" --project 1
gsd --db .sandbox/demo12.db add "Water the plants" --area 1
gsd --db .sandbox/demo12.db tui
    # Areas → Home (projects, then loose tasks) → Kitchen reno;
    # Esc ×3; Boards → software (stage groups with progress);
    # Esc ×2; Areas → (no area); q
```

- [x] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database,
      capture the frames into the deck, and pass local `make check`.

## Chunk 3 — Every door opens

Human outcome: walk from the root to any entity's detail and back out
with Esc; mutate via the CLI mid-session and see it on re-entry —
freshness proven live.

Implementation:

- [x] `internal/tui/navigator` detail view: one renderer for task,
      project, area, and board mirroring `show`'s field order per the
      settled design; empty fields collapse; notes escaped with line
      feeds preserved; the promotes marker matches the CLI.
- [x] Enter/`l` wiring: task, project, and area rows in every list —
      including logbook rows — open that entity's detail; container
      headers open the container's detail; detail has no cursor and
      no filter; Esc pops.
- [x] Data: `task.Show`, `project.Show` (with the board/stage
      location), `area.Show`, `board.ShowByID`.

Verification (primary owner: navigator model tests with fake
dependencies):

- [x] Model: field order and values per kind — the project's
      `Board/Stage` row, defer-stage and promotes rows, the board's
      stage list — with empty-field collapse and control-character
      escaping in notes.
- [x] Model: Enter/`l` targets rows versus headers correctly; logbook
      rows open task and project details; an entity deleted
      mid-session renders its `not_found` inline and Esc backs out.
- [x] Model: pop-return re-calls the list loader, so a mutated entity
      shows its new state.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/12-chunk-3.html`), exact
commands:

```sh
gsd --db .sandbox/demo12.db tui
    # Areas → Home → header Enter: area detail; Esc;
    # Kitchen reno → header Enter: project detail; Esc;
    # a task Enter: task detail; leave the session open
gsd --db .sandbox/demo12.db edit 2 --note "under the sink"
    # from a second terminal, mid-session
    # back in the TUI: Esc, re-enter the task — the note is there; q
```

- [x] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database,
      capture the frames into the deck, and pass local `make check`.

## Chunk 4 — The lights come on

Human outcome: the navigator wears a deliberate design language — a
badged breadcrumb band above, a key-hint band below, Picker-style
filled selection, glyph-led record rows, and urgency accents — in
both Catppuccin themes, in a full terminal and a tmux popup.

Settled with the human over live captures, 2026-08-15. The design
language is **Picker with the capture badge** (fzf/telescope lineage
grafted onto the settled quiet-chrome vocabulary). Where this record
conflicts with the 2026-08-08 settled design above, this chunk
supersedes it; the superseded points are called out inline.

- **Chrome.** Top band on `InputBg`: the ` gsd ` badge
  (`AccentText` on `Accent`, capture's idiom) then a breadcrumb of
  entity titles — dim parents, bold current, `…` left-collapse when
  narrow, adjacent duplicate segments merged (a container header's
  detail does not repeat the title). The root shows the badge alone.
  Bottom band: dim per-view key hints pinned to the terminal's last
  row (`j/k move · ⏎ open · esc quit|back`; `j/k scroll · esc back`
  on detail; `esc back` alone on a failed view). The bottom band is
  the designated home of chunk 5's `/` filter, Milestone 15's `:`
  command line, and Milestone 14's mutation feedback. Content gets
  height minus three; the spacer under the top band collapses first;
  below three rows the bands drop entirely. No-color mode renders
  identical text unstyled.
- **Selection.** An accent `▌` edge plus an `InputBg` fill across
  the full row width; plain cells brighten to `Text`, accent-colored
  cells keep their color. Supersedes the marker-only selection
  ("accent marker without styling the whole row") pinned at
  Milestone 10.
- **Record rows.** Every list row is glyph · title · dim trailing
  annotations, the same anatomy as detail headlines: `•` task, `◆`
  project, `●` area, `○` the all-dim `(no area)` pseudo-row;
  logbook rows recolor the glyph (`✓` green, `✗` red) and trail dim
  kind and date; board collection rows trail the dim stage chain;
  board container rows trail dim progress. Column-header rows and
  contextually invariant columns (collection `state`, container
  `open`) are gone. Ids appear only in detail views — the headline
  is glyph + title, `id` leads the field list, and breadcrumbs use
  titles. Supersedes "task rows mirror the CLI's list columns".
- **Urgency.** Due dates render `Yellow` (new theme token: Latte
  `#df8e1d`, Frappé `#e5c890`); an open task due on or before today
  (local time, injected clock) renders bold red, matching the CLI's
  overdue rule.
- **Containers.** Blank line after the selectable header, blank line
  between sections, rows indented two cells under titled section
  headings; unnamed sections stay flush. The `(no area)` container
  dropped its plain non-selectable title — the breadcrumb names the
  view — and the plain-title rendering machinery is deleted.
- **Board modes.** The vertical stage-grouped view keeps the
  container treatment ("Quiet" variant): boards gain the `▥` glyph
  on the collection row, container header, and detail headline, and
  empty stages render as bare dim headings — `(empty)` is
  board-only suppressed, area sections keep it. The horizontal
  column/card mode is Milestone 13's capability and inherits this
  language; its plan gate receives the settled mock (columns are
  stages, two-line cards of title over dim progress, Picker fill on
  the selected card, bold heading on the selected column, unchanged
  bands), recorded in `MILESTONE_13.md`. The fully tree-based
  operating mode (`l` expand / `h` collapse over one tree) is the
  optional Milestone 19.

Implementation:

- [x] `internal/tui`: `Yellow` theme token (Latte/Frappé pair).
- [x] `internal/tui/navigator`: top/bottom bands with height
      budgeting; breadcrumb crumbs carried on rows and frames,
      refreshed from container headers on reload; per-view hints.
- [x] Picker selection fill; per-cell accent styling that survives
      the fill; record-row builders; column-header removal.
- [x] Detail views: glyph + title headline, `id` as first field,
      title breadcrumbs with duplicate collapse.
- [x] Container spacing (breathing room + indent) with width math
      accounting for the indent.
- [x] Polish: `○ (no area)` pseudo-row, error-view hints, Latte
      verified against a live light-background terminal.
- [x] `.sandbox/design/demo.sh` (seeded human-testing entry) and
      `.sandbox/design/capture-state.sh` (regenerates
      `current-state.html` from the real binary).
- [x] Board vertical treatment ("Quiet": `▥` glyph, bare empty
      stages).
- [x] Colored accent coverage: urgency dates and logbook glyphs.
- [x] Chunk demo deck (`.sandbox/demos/12-chunk-4.html`), rebuilt
      from the current-state frames by
      `.sandbox/design/build-demo.py`.

Verification (primary owner: navigator model tests):

- [x] Exact-output pins updated: framed root, bands, selection fill
      and theme swap, record rows, detail field order, container
      spacing, `(no area)` treatments, error hints.
- [x] `make check` green at every human checkpoint.
- [x] Final agent-verified tmux pass
      (`.sandbox/design/verify-chunk4.sh`): root chrome and rows,
      areas → area → project → task detail drill with breadcrumbs,
      id-as-field, esc cursor restore, mid-session CLI edit
      reflected on re-entry, board glyph/stages/bare-empty, board
      detail, `q` exit 0, `--json` and positional-argument guards
      exit 2 — all passing against a fresh database.

## Chunk 5 — Type to find

Human outcome: `/` plus a few characters narrows any view to the
thing you meant; clearing restores the view exactly.

Implementation:

- [ ] `go.mod`: `github.com/sahilm/fuzzy` becomes a direct dependency
      via `make tidy`.
- [ ] `internal/tui/navigator` matcher: fuzzy subsequence wrapper
      with the smart-case rule, returning the matched set and match
      positions; scores stay internal until a ranked surface needs
      them.
- [ ] Filter mode on list views: `/` opens the input band; each
      keystroke re-scores the view's visible row text; non-matches
      hide, sections and headers stay, matched characters highlight,
      the cursor clamps to the matched set; a blank pattern is
      unfiltered; Esc clears the filter before navigating; any
      navigation drops it; Enter/`l` on a filtered row behaves normally.
- [ ] `e2e/navigator_test.go`: the documented end-to-end workflow
      below as durable subprocess coverage inside `make check`.

Verification (primary owners: matcher unit tests; navigator model
tests; e2e for the complete workflow):

- [ ] Matcher: subsequence semantics (`plmb` matches
      `Call plumber`), the smart-case rule, and position reporting;
      scores are not pinned.
- [ ] Model: per-keystroke narrowing; headers and sections persist;
      clearing restores rows and cursor; Esc clears then backs;
      descending drops the filter; root, collections, and containers
      all filter; detail does not.
- [ ] e2e: the full workflow passes against the real binary.
- [ ] `make check` green.

Human proof (chunk demo `.sandbox/demos/12-chunk-5.html`), exact
commands:

```sh
gsd --db .sandbox/demo12.db tui
    # Available; / then "plmb" — the plumber task remains;
    # Esc clears; Esc back; Areas → Home; / "reno" —
    # Kitchen reno remains; q
```

- [ ] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database,
      capture the frames into the deck, and pass local `make check`.

## Agent-verified end-to-end workflow

Run tmux-driven against the real built binary and a seeded temporary
database after all chunks merge (`PROCESS.md` step 4); the equivalent
durable coverage lives in `e2e/` inside `make check`:

1. The root shows Inbox, Available, Logbook, Boards, Areas in order;
   Boards lists the seeded board with its stage chain; Areas lists
   active areas in position order with `(no area)` last.
2. Drill Areas → area → project → task detail and back out with Esc
   at each level; each screen contains the expected rows and fields;
   the cursor lands back where it was.
3. Enter/`l` on container headers: area, project, and board details match
   `show --json` (and `board show --json`) fields for the same
   identities.
4. Mutate via the CLI mid-session — edit a note, move a boarded
   project; re-entering the affected views reflects the change.
5. `/` narrows the current view per keystroke by fuzzy subsequence;
   clearing restores the view; Esc clears the filter before it
   navigates back.
6. Guards: positional arguments, `--json`, and non-TTY invocation are
   usage errors (exit 2) naming the CLI; `--help` never opens the
   database; `q` and Esc at the root exit 0.

Report the clean transcript.

## Consolidation checklist (`PROCESS.md`)

- [ ] `COMMANDS.md`: `tui` documented as a shipped command — the
      two-lens tree, the three view shapes, keys, freshness, and the
      fuzzy `/` semantics; the TUI section's root-tree and search
      bullets reconciled (resolving D-004); `tui` joins `config` and
      `capture` as the `--json`-rejecting exceptions.
- [ ] `SCHEMA.md`: no changes expected from this read-only milestone —
      confirm.
- [ ] `OVERVIEW.md`: the TUI navigation vocabulary (lens, view stack,
      collection/container/detail) where the concept section needs
      it.
- [ ] `DIVERGENCES.md`: D-004 removed at reconciliation.
- [ ] Carried items re-homed per `PROCESS.md`; the dispositions above
      recorded in the next milestone's file at wrap-up.
- [ ] Standard exit workflow plus `MILESTONE_12.md`'s exit criteria.
