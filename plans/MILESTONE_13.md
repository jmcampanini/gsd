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
