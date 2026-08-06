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
the test seams. Milestones 11–13 build on that foundation only after it
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

- [ ] `COMMANDS.md` documents `capture` as a shipped command.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003; TUI-applicable standards re-checked
at plan gate.
