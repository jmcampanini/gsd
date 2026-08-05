# Milestone 14 — Row verbs

Data mode: **live**. Depends on: Milestone 13; the reorder chunk rides
Milestone 7's operations.

Written light; re-review at plan gate.

## Capability

The mutation layer: act on what you're looking at without leaving the
TUI. Single-key verbs apply to the selected row, feedback renders
inline, and the view refreshes after every write. This layer — verbs,
confirm, feedback, refresh — is the foundation the command line and
richer capture build their result rendering on.

## Scope

- Single-key mnemonics on the selection, shorthand for the same CLI
  verbs, never a second vocabulary: `a` add (title-only quick add into
  the current container, reusing capture's input model), `d` done, `x`
  cancel, `o` reopen, `D` delete with confirm. The exact set is settled
  at plan gate; `e` edit and `t` tag are deliberately deferred to
  Milestone 15, where the grammar surface exists to express them.
- Inline feedback reuses the glyph-led mutation vocabulary; application
  errors render inline with the shared accents.
- The active view refreshes after every mutation; cascades (project
  done/cancel) narrate through the same feedback surface.
- **Reorder chunk**: grab-and-move on the selection, pure presentation
  over Milestone 7's reorder operations — no new ordering model.
  Drop-commit semantics: one reorder call per completed drop, no
  intermediate persistence while dragging; multi-select reordering is out
  of scope unless a later milestone deliberately adds it.

## Chunks

1. **Resolution verbs** — `d`/`x`/`o` with feedback and
   refresh-after-mutation.
2. **Add and delete** — `a` quick add into the current container; `D`
   with confirm, cascade narration.
3. **Reorder** — grab-and-move driving the Milestone 7 operations.

## Agent-verified end-to-end workflow

Tmux-driven against the real binary and a seeded temporary database:
complete/cancel/reopen tasks and projects from the list, add into a
project, delete with confirm, grab-and-move a row; verify every
mutation via `--json` CLI reads and that each view refresh reflects it.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)).

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003; TUI-applicable standards re-checked
at plan gate.
