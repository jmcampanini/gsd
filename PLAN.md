# Milestone 13 — Board view: root implementation plan

[`plans/MILESTONE_13.md`](plans/MILESTONE_13.md) is authoritative for
this milestone's outcomes and acceptance boundary;
[`plans/PROCESS.md`](plans/PROCESS.md) governs workflow, review, and
consolidation. This plan is temporary and is retired at consolidation.

## Progress

- [ ] Chunk 1 — The board turns sideways
- [ ] Chunk 2 — Cards open and stay fresh

There is no chunk 0: the Milestone 12 foundation review scheduled
nothing; the deferred items carry forward in `MILESTONE_13.md` with
their revisit triggers, and none of their triggers fire in this
milestone (no date-grammar change, no third input mode, no new
`search.Hit` consumer, no broad pin rewrite, no TUI write path; the
WIP-limit trigger needs real over-commitment pain, not the view
merely existing).

## Settled design

Plan-gate interview, 2026-08-15/16, settled over an interactive
visual mock (`.sandbox/mocks/milestone-13-board-columns.html`, kept as
the uncommitted visual reference). The governing decision: **the
vertical board view stays the door; the column view is a deeper
screen.** One correction to the milestone file: board rows have been
enterable since Milestone 12 — they open the vertical stage-grouped
container, not inert rows — so entry composes with shipped behavior.

- **Coexistence and entry.** Enter on a board row keeps opening
  today's vertical view, unchanged. From anywhere in the vertical
  board view, `v` pushes the column view onto the navigation stack;
  Esc pops back to the vertical view. Both renderings persist; a more
  ergonomic swap gesture is deliberately deferred until real usage
  teaches what to want. `v` is bound only in the vertical board view.
  The pushed frame's crumb is the board title, which the breadcrumb's
  duplicate-segment merge collapses, so the top band still reads
  `Boards ▸ <board>`.
- **One view kind, no new machinery.** The column view is a new view
  kind inside the navigator's view stack — its own layout renderer
  and a two-dimensional cursor (column, card), reusing the existing
  breadcrumb, cursor-memory, load-on-entry, and inline-error
  machinery. Data comes from the existing `BoardReader.ShowByID`
  (`board.Show`): stages in position order, open projects in stage
  position order, derived done/total progress. No store or service
  work; the milestone stays read-only — stage movement arrives with
  Milestone 14's stage-movement chunk.
- **Keys.** `h`/`l` and `←`/`→` cross columns, clamping at the first
  and last column; `j`/`k`/`↑`/`↓` walk cards; Enter opens the
  selected card's **project detail** (the uniform detail view — a
  deliberate divergence from the vertical view, where Enter descends
  into the project container); Esc pops; `q` and Ctrl+C quit.
  `←`/`→` remain unbound everywhere else. Crossing columns keeps the
  card *index*, clamped to the destination column's length. Empty
  columns are selectable — the heading bolds, no card is selected,
  Enter is inert — so `h`/`l` never skip a stage. Column-view keys
  live in the existing navigation branch of `updateKey`, dispatched
  by view kind; no third input mode exists, so that carried trigger
  does not fire.
- **Layout.** Content indents two cells. Columns share the width
  equally with a 20-cell floor and three-cell gutters. When the
  stages don't fit, a horizontal viewport follows the selected
  column and **only whole columns render** — the visible columns
  stretch equally to consume the full width, and nothing is ever
  clipped mid-column. Card text truncates with `…` and never wraps.
- **Separators.** Each gutter carries a faint `│` rule the full
  content height; a faint `─` underline spans each column's width
  under the heading row. Neither takes the accent color — selection
  reads through the bold heading and the card fill alone.
- **Headings and cards.** Stage headings are dim; the selected
  column's heading renders bold in the text color. Cards are two
  lines — project title over dim `done/total` — with a blank line
  between cards; the selected card takes the accent `▌` edge and the
  row fill across the card's column width. Empty stages render as
  bare headings, matching the vertical view.
- **Edge signals.** When columns overflow, hidden-column **counts**
  render accent-bold in the heading-row corners (`‹2` left, `3›`
  right) and a dim full-height `░` **bar** runs down each
  overflowing screen edge. Both appear only on a side that actually
  hides columns. Uncolored output renders the same characters
  unstyled; structure is identical.
- **Card overflow.** Every column renders from its top; only the
  selected column scrolls vertically to keep the selected card
  visible.
- **Cursor memory.** Returning to the column view — popping back
  from detail, or re-entering after a CLI mutation — restores the
  selection by project identity searched board-wide, so a project
  `move`d to another column mid-session is found in its new column;
  when it is gone, the cursor falls back to the clamped column and
  card index.
- **`/` filters cards; columns stay fixed.** The existing fuzzy
  matcher and edit/commit/clear gestures apply unchanged: cards
  filter by their visible text with matched characters highlighted,
  non-matching cards hide, every column and heading stays in place,
  and the cursor clamps to the surviving cards. Esc peels one layer
  per press — editing → filtered → cleared → back. The filter is
  view-local and drops on any navigation.
- **Freshness.** The existing paradigm: load on entry, re-read on
  re-entry including pop-return, no polling.
- **Chunk boundary.** Settled at this gate, superseding the
  milestone file's suggestive chunking: chunk 1 delivers the
  scaffold and the *complete look* — layout, floor-and-viewport,
  separators, edge signals, card scrolling, navigation, empty
  stages, `v` entry — with Enter and `/` deliberately inert; chunk 2
  delivers the interactions — detail round trip, `/` filter,
  freshness proof — plus the durable end-to-end coverage and the
  documentation item.
- **Test ownership.** Navigator model tests with fake readers are
  the primary owner of layout math, key semantics, cursor behavior,
  and exact-frame pins (inline pins per the golden-frame deferral —
  new pins are not a broad rewrite). The tmux e2e test owns
  real-binary entry, the detail round trip, and freshness after a
  CLI mutation. No command-layer changes are expected: `gsd tui`'s
  guards, color chain, and factory wiring are untouched.

## Chunk 1 — The board turns sideways

Human outcome: from a board's vertical view, `v` opens the
full-screen column view wearing the complete settled look — floored,
stretched whole columns with rule and underline separators, counts
and bars when stages overflow, per-column card scrolling — navigable
with `h`/`l`/`←`/`→` and `j`/`k`; Esc returns to the vertical view.

Implementation:

- [ ] `internal/tui/navigator`: the column view kind — frame state
      for the two-dimensional cursor and horizontal viewport offset,
      loading via `BoardReader.ShowByID`, crumb set to the board
      title (merged by the breadcrumb), inline error state inherited.
- [ ] Layout renderer: equal-share widths with the 20-cell floor and
      three-cell gutters; whole-columns-only viewport following the
      selected column, visible columns stretched to fill; two-line
      cards with the accent-edge selection fill; dim/bold headings;
      bare empty stages; `│` rules and `─` underlines; counts and
      `░` bars on overflowing sides only; per-column vertical scroll
      following the selected card; `…` truncation throughout.
- [ ] Keys: `v` in the vertical board view pushes the column view;
      `h`/`l`/`←`/`→` and `j`/`k`/`↑`/`↓` with clamped-index column
      crossing and selectable empty columns; Enter and `/` inert
      this chunk; Esc pops; dispatch stays inside the existing
      navigation branch, per view kind.
- [ ] Bottom band: column-view key hints
      (`h/l columns · j/k cards · esc back`); the vertical board
      view's hints gain `v columns`.

Verification (primary owner: navigator model tests with fake
dependencies):

- [ ] Layout math: all stages fit at or above the floor when the
      width allows; below that, only whole columns render, stretched
      to consume the width, at every width down to a single column —
      never a partial column.
- [ ] Exact-frame pins, colored and uncolored: rules, underlines,
      selected-column bold heading, selected-card fill, empty stage,
      counts and bars present only on sides hiding columns.
- [ ] Navigation: clamped index carry across columns; empty columns
      selectable with inert Enter; ends clamp; the viewport follows
      the selected column; the selected column scrolls to its card
      while others stay pinned to top.
- [ ] Entry: `v` pushes only from the vertical board view (inert
      elsewhere); Esc pops back with the vertical view's cursor
      restored; the breadcrumb shows `Boards ▸ <board>` unduplicated.
- [ ] `make check` green.

Human proof (chunk demo `.sandbox/demos/13-chunk-1.html`; pane frames
captured per the navigator precedent), exact commands:

```sh
gsd --db .sandbox/demo13.db boards add software --stage backlog \
    --stage next --stage doing --stage review --stage later
gsd --db .sandbox/demo13.db projects add "Kitchen reno" --board software
gsd --db .sandbox/demo13.db projects add "Blog redesign" --board software
gsd --db .sandbox/demo13.db projects add "Taxes 2024" --board software
gsd --db .sandbox/demo13.db project move 1 doing
gsd --db .sandbox/demo13.db project move 2 next
gsd --db .sandbox/demo13.db add "Demo cabinets" --project 1
gsd --db .sandbox/demo13.db add "Pick paint" --project 1
gsd --db .sandbox/demo13.db done 1
gsd --db .sandbox/demo13.db tui
    # Boards → software (the vertical view, unchanged); v: columns;
    # h/l across all five stages including empty "later"; j/k in
    # "backlog"; in a narrow pane: counts + bars appear, whole
    # columns only; Esc: the vertical view; Esc ×2; q
```

- [ ] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database —
      including a narrow-pane frame proving the overflow treatment —
      capture the frames into the deck, and pass local `make check`.

## Chunk 2 — Cards open and stay fresh

Human outcome: Enter on a card opens the project's detail and Esc
lands back on the same card; `/` narrows the cards while the columns
hold still; a CLI move or completed task from another terminal shows
up on the next entry — the whiteboard stays honest.

Implementation:

- [ ] Enter on the selected card pushes the uniform project detail;
      cursor restoration by project identity searched board-wide,
      with the clamped column/card fallback when the project is
      gone.
- [ ] `/` filter over card text through the existing matcher: cards
      hide, match positions highlight, columns and headings stay,
      the cursor clamps to the matched set; the edit → commit →
      clear Esc peeling and view-local drop-on-navigation semantics
      identical to the list views.
- [ ] Column-view hints gain `⏎ open · / filter`; the filter band
      hosts the query as on every other view.
- [ ] `e2e`: the documented end-to-end workflow below as durable
      subprocess coverage inside `make check`, on the shared tmux
      harness.

Verification (primary owners: navigator model tests; e2e for the
complete workflow):

- [ ] Model: Enter opens the detail for the selected card and is
      inert on an empty column; popping back restores the same
      card; after the fake reader moves a project across stages,
      re-entry finds it in its new column; a vanished project falls
      back to the clamped position.
- [ ] Model: per-keystroke narrowing over card text; empty-after-
      filter columns keep their headings; Enter opens from editing
      and committed modes; Esc peels editing → filtered → cleared →
      back; any navigation drops the filter; the filter never moves
      or hides a column.
- [ ] Model: pop-return and re-entry re-call the loader, so new
      stage placement and progress counts render.
- [ ] e2e: the full workflow passes against the real binary.
- [ ] `make check` green.

Human proof (chunk demo `.sandbox/demos/13-chunk-2.html`), exact
commands:

```sh
gsd --db .sandbox/demo13.db tui
    # Boards → software; v; l to "doing"; Enter: Kitchen reno's
    # detail; Esc: back on the same card; / "blog" — the matching
    # card remains, all five columns stay; Esc ×2: cleared;
    # leave the session open
gsd --db .sandbox/demo13.db project move 2 review
gsd --db .sandbox/demo13.db done 2
    # from a second terminal, mid-session
    # back in the TUI: Esc to the vertical view, v again —
    # Blog redesign sits in "review" and the progress updated; q
```

- [ ] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database,
      capture the frames into the deck, and pass local `make check`.

## Agent-verified end-to-end workflow

Run tmux-driven against the real built binary and a seeded temporary
database after all chunks merge (`PROCESS.md` step 4); the equivalent
durable coverage lives in `e2e/` inside `make check`:

1. The root tree lists Boards; Enter opens the seeded board's
   vertical view; `v` opens the column view with columns in stage
   order and cards matching `board show --json` for the same board —
   titles, stage grouping, position order, and progress counts.
2. Navigate across columns and cards, including the empty stage;
   Enter opens the correct project detail; Esc returns to the same
   column and card.
3. In a narrow pane, only whole columns render with counts and bars
   on the overflowing side; `h`/`l` walks the viewport across all
   stages.
4. `/` narrows the cards per keystroke while every column and
   heading stays; Esc peels the filter one layer per press.
5. Move a project and complete a task via the CLI mid-session;
   re-entering the column view shows the new column placement and
   progress count.
6. Esc unwinds column view → vertical view → Boards → root; `q`
   exits 0.

Report the clean transcript.

## Consolidation checklist (`PROCESS.md`)

- [ ] `COMMANDS.md`: the TUI section documents the board column view
      (the milestone's exit criterion) — `v` entry from the vertical
      view, the column/card key vocabulary, whole-columns overflow
      with counts and bars, rule and underline separators, the card
      filter, and Enter-to-detail; the "still planned" board-column
      bullet moves into shipped behavior.
- [ ] `OVERVIEW.md`: the TUI navigation paragraph names the column
      view beside the vertical board view.
- [ ] `SCHEMA.md`: no changes expected from this read-only milestone
      — confirm at consolidation.
- [ ] `DIVERGENCES.md`: none expected; record and resolve any
      discovered contradiction per protocol.
- [ ] Carried items re-homed per `PROCESS.md`: the still-open
      deferrals move to `MILESTONE_14.md` with their triggers at
      wrap-up, plus the new usage-driven revisit — a more ergonomic
      swap between the two board renderings.
- [ ] Standard exit workflow plus `MILESTONE_13.md`'s exit criteria.
