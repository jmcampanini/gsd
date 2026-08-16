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

## Carried from the Milestone 12 foundation review

Recorded at finding triage; the wrap-up re-homing pass merges these
with the rest of the carried items.

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

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003; TUI-applicable standards
re-checked at plan gate.
