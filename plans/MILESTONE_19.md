# Milestone 19 — Tree

Data mode: **live**. Depends on: Milestone 12 (navigation layer and
design language); sequenced after Milestone 13 (board view) so both
lenses exist to hang from the tree. **Optional**: enters planning
only when its trigger fires.

Written light; re-review at plan gate on activation.

## Capability

A second operating mode for the TUI: the whole system as one
expandable tree — root rows, lenses, containers, and leaves in a
single view — navigated and folded entirely with `hjkl`, as an
alternative to the drill-in/drill-out view stack.

## Scope

- One tree over the existing levels: Inbox, Available, Logbook,
  Boards, and Areas at the top; boards and areas expand to their
  projects; projects expand to their tasks. Same objects, same
  services, no new data surface.
- Folding: `l` expands the selected node (or opens a leaf's detail),
  `h` collapses it (or moves to the parent when already collapsed);
  `j`/`k` move linearly through visible nodes; Enter opens the
  uniform detail view. Collapse state is per-session.
- Collapsed nodes carry a dim `▸`, expanded a dim `▾`, ahead of the
  record-row glyphs; everything else inherits the Milestone 12
  design language unchanged — bands, selection fill, record rows.
- Plan-gate decisions on activation: the mode entry/exit spelling (a
  root row, a key, or a Milestone 15 command-line verb), how the
  tree coexists with the view-stack mode, freshness semantics for
  expanded nodes, and whether `/` filters visible nodes.

## User stories

```text
$ gsd tui        # tree mode: ▸ Inbox … ▸ Boards ▸ Areas
                 # l on Areas: ▾ Areas, areas nested beneath
                 # l on Home: its projects and loose tasks nested
                 # j to a task, Enter: detail; Esc: back in the tree
                 # h twice: collapse Home, then Areas
```

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)); re-derived
at plan gate on activation.
