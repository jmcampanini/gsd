# Milestone 10 — Capture

Data mode: **live**. Depends on: Milestone 9 (live baseline).

Written ahead of Milestone 9 landing; re-review at plan gate.

## Capability

`gsd capture` is the first charm surface: a popup-sized single-input
program that turns typed text into an inbox task and vanishes — built to
live inside `tmux display-popup`. It is deliberately the thinnest
vertical slice through everything the TUI will stand on, so this
milestone's real deliverable is the reviewed substrate: the Bubble Tea
v2 embedding, the program lifecycle from a cobra command through the
injected factory, the theme bridge onto the settled visual system, and
the test seams. Milestones 12–15 build on that foundation only after it
has been consolidated and foundation-reviewed here.

## Scope

### Command

```text
gsd capture
```

- One text field. A non-empty title plus Enter creates an open task with
  exactly `gsd add TITLE` semantics — no other fields — then exits `0`
  with no output; the popup just vanishes. Esc exits `0` without
  writing. Enter on an empty field is a no-op (**proposed**).
- Interactive-only: `capture` requires a terminal and takes no
  arguments; `--json` is a usage error, like `config`. Non-TTY
  invocation is a usage error (**proposed**).
- The full config chain applies (`--config`, `--db`, `GSD_DB`, TOML,
  defaults); the database opens through the injected factory only when
  the program starts — help and parse failures never touch it.
- A failed write renders its application error inline, styled with the
  shared accents; the program then exits `1` on dismiss (**proposed**).
- Keyboard-only; no mouse.

### Substrate

- `charm.land/bubbletea/v2` (plus `bubbles/v2` where a stock component
  fits) joins the existing charm.land v2 line — lipgloss v2 and the
  ultraviolet renderer are already in the tree.
- A new `internal/tui` package owns presentation: program construction
  from a cobra command (streams, terminal handling, alt-screen policy),
  model code consuming the `Application` interfaces (capture needs
  `task.Application` only), and a theme bridge reusing the
  background-adaptive Catppuccin accents and per-stream color rules
  (CLI-OUTPUT-001; `NO_COLOR` honored).
- cmd stays adapter-thin per `AGENTS.md`: the command adapts flags and
  streams and hands the factory's applications to the program.
- Test ownership: model behavior (input, submit, cancel, error render)
  with fake applications; command-layer wiring and guard tests in
  `cmd`; a subprocess e2e proving a real popup write persists across
  invocations.

## Chunks

1. **Capture works** — dependency, program lifecycle, minimal input
   model; Enter writes through `task.Add`, Esc cancels. Human proof: a
   `tmux display-popup` capture lands in `gsd inbox`.
2. **Capture contract** — theme bridge styling, empty-input and
   write-error handling, TTY/`--json` guards, subprocess e2e, and the
   tmux-driven agent workflow.

## Carried from Milestone 9

The Milestone 9 foundation review's fix-now findings were resolved on
the milestone branch; nothing was scheduled, so there is no chunk 0
work. The deliberately deferred items carry forward with their revisit
triggers:

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
- **Typed transition spec for `applyTransition`** — carried from
  Milestone 6: revisit if post-v1 work adds transitions.
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

### Capture from anywhere

```text
$ tmux display-popup -w 64 -h 3 -E 'gsd capture'
> Call plumber█
$ gsd inbox
  4  Call plumber
```

### Cancel is free

```text
$ tmux display-popup -w 64 -h 3 -E 'gsd capture'   # Esc
$ gsd inbox
  4  Call plumber                                   # unchanged
```

## Agent-verified end-to-end workflow

Against the real built binary and a temporary database, driving a
popup-sized tmux pane with send-keys:

1. Launch `gsd capture`, type a title, Enter; the program exits `0` and
   `gsd inbox --json` shows the task with only the title set.
2. Esc path: launch, type, Esc; exit `0`, no row added.
3. Config chain: the capture lands in the database selected by `--db`
   and by `GSD_DB`.
4. Guards: `--json` and non-TTY invocation are usage errors, exit `2`;
   help never opens the database.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [x] `COMMANDS.md` documents `capture` as a shipped command.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003; TUI-applicable standards re-checked
at plan gate.
