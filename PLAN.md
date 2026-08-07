# Milestone 10 — Capture: root implementation plan

[`plans/MILESTONE_10.md`](plans/MILESTONE_10.md) is authoritative for
this milestone's outcomes and acceptance boundary;
[`plans/PROCESS.md`](plans/PROCESS.md) governs workflow, review, and
consolidation. This plan is temporary and is retired at consolidation.

## Progress

- [x] Chunk 1 — Capture works
- [x] Chunk 2 — Capture contract

There is no chunk 0: the Milestone 9 foundation review scheduled
nothing; the deferred items carry forward in `MILESTONE_10.md` with
their revisit triggers.

## Settled design

Plan-gate interview, 2026-08-07. The milestone's three proposed
defaults are ratified as written: Enter on an empty field is a no-op,
non-TTY invocation is a usage error (exit 2), and a failed write
renders its application error inline with exit 1 on dismiss.

- **Dependencies.** `charm.land/bubbletea/v2` and
  `charm.land/bubbles/v2` (stock `textinput`) join the charm.land v2
  line already in the tree.
- **Package.** One flat `internal/tui` package owns the substrate:
  program construction, the theme, and the capture model. Subpackages
  split out only when Navigator (Milestone 11) adds real bulk.
- **Program construction.** The constructor takes the cobra command's
  streams and a per-program screen mode; capture runs in alt-screen
  (Navigator will reuse the same option). The command computes the
  color decision with the existing chain (`--color` > nonempty
  `NO_COLOR` > per-stream auto, via `resolveColor`) and hands it to
  the program; a disabled decision selects the plain style set, so the
  surface stays functional uncolored. Light/dark is detected
  in-program via `tea.RequestBackgroundColor` (lipgloss v2 has no
  `AdaptiveColor`), defaulting to dark until the response arrives,
  matching the CLI's existing default.
- **Theme.** The green/red `LightDark` pairs move from
  `cmd/output.go` into the `internal/tui` theme and `cmd` imports them
  from there; CLI rendering is unchanged. The theme adds cmdk-style
  chrome tokens — `Accent` is Catppuccin Mauve (Latte `#8839ef` /
  Frappé `#ca9ee6`), plus `AccentText`, `InputBg`, `Text`, and a dim
  token drawn from the matching Latte/Frappé entries (cmdk as
  reference). `Cursor` is optional and defaults to nil so the terminal
  retains control of its cursor color. Green/red remain state accents
  only.
- **View.** Borderless vertical stack per Bubble Tea's own
  single-input example — the tmux popup frame is the border. A `gsd`
  badge (`AccentText` on Mauve) renders as the input prompt, the input
  line is composed over a view-level `InputBg` band so the real cursor
  cannot punch through it, there is no placeholder, and a faint footer
  reads `enter add · esc cancel`. The footer row makes the recommended
  invocation `tmux display-popup -w 64 -h 4` (the milestone mock's
  `-h 3` leaves one content row inside tmux's frame).
- **Behavior.** Enter with a non-blank title calls `task.Add` with the
  title verbatim — untrimmed, no other fields, exactly `gsd add TITLE`
  semantics — then exits 0 with no output. Blank input (the domain's
  definition: `TrimSpace` empty) makes Enter a no-op. Esc and Ctrl+C
  both cancel: exit 0, nothing written. A failed write renders its
  application error inline styled with the red accent; any key
  dismisses, the command returns the error through
  `normalizeApplicationError`, and the root adapter emits the standard
  stderr line with exit 1.
- **Guards.** In `RunE` before `withTaskApplication`, so usage errors
  never open the database: `--json` is refused like `config`; stdin
  and stdout TTYs are checked separately (the capability actually
  required) through the `presentationDependencies.isTerminal` seam,
  generalized to cover the input stream. The non-TTY message names
  `gsd add` as the noninteractive recovery path. `capture` takes no
  arguments.
- **Wiring.** The command reaches the service through the existing
  `withTaskApplication` wrapper: lazy open at command execution,
  deterministic close, error normalization. Help and parse failures
  never touch the database, preserving the property the e2e suite
  already checks.
- **Interactive e2e.** A tmux-driven harness inside `make check`: a
  private tmux server per test (isolated socket under the e2e work
  directory), a popup-sized session running the real binary,
  `send-keys` input, persistence verified by a plain subprocess
  invocation. tmux is a documented external prerequisite: `check.yml`
  installs it, the README documents it, and the e2e fails loudly when
  it is absent rather than skipping.
- **Standards, re-checked at this gate.** The interactive-cli pattern
  holds: capability checks are split per stream, the interactive-only
  refusal is actionable, and the external prerequisite is exercised in
  CI. CLI-OUTPUT-001 governs the color chain; CLI-OUTPUT-003 is
  vacuous (`--json` is refused); CLI-OUTPUT-004 is the non-TTY guard.

## Chunk 1 — Capture works

Human outcome: a `tmux display-popup` capture lands in `gsd inbox` —
type a title, Enter, and the popup vanishes with the task created; Esc
vanishes without writing.

Implementation:

- [x] `go.mod`: add `charm.land/bubbletea/v2` and
      `charm.land/bubbles/v2` via `make tidy`.
- [x] `internal/tui` theme: green/red pairs extracted from
      `cmd/output.go` plus the Mauve chrome tokens and optional cursor
      token; light/dark selection driven by `tea.RequestBackgroundColor`.
- [x] `cmd/output.go`: consume the extracted pairs from
      `internal/tui`; rendering byte-identical (existing SGR
      expectations in `cmd/output_test.go` stay green).
- [x] `internal/tui` program constructor: streams from the cobra
      command, per-program screen mode, capture in alt-screen.
- [x] `internal/tui` capture model: badge + view-level input band +
      `textinput` + footer view; Enter submits non-blank input through
      `task.Application.Add`
      (title verbatim, no other fields) and quits; blank Enter is a
      no-op; Esc and Ctrl+C quit without calling `Add`.
- [x] `cmd/capture.go`: `gsd capture` registered on the root command,
      running the program through `withTaskApplication` with the
      command's streams and color decision.

Verification (primary owner: `internal/tui` model tests with a fake
`task.Application`; command wiring in `cmd`):

- [x] Typing updates the input; Enter with a non-blank title calls
      `Add` exactly once with the verbatim title and no other fields,
      then quits cleanly.
- [x] Blank input (empty and whitespace-only): Enter is a no-op — no
      `Add` call, program still running.
- [x] Esc and Ctrl+C quit without calling `Add`.
- [x] Theme selects Latte/Frappé values by background, keeps the
      terminal-default cursor unless a token is set, maintains a
      continuous input band, and uses plain styles when color is disabled.
- [x] `cmd`: capture receives the factory's applications and the
      command streams; the factory opens once at run and never for
      `--help`.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/10-chunk-1.html`; popup frames
captured from an attached tmux client while open), exact commands:

```sh
tmux display-popup -w 64 -h 4 -E 'gsd --db .sandbox/demo.db capture'
                                    # type "Call plumber", Enter
gsd --db .sandbox/demo.db inbox     # the task landed
tmux display-popup -w 64 -h 4 -E 'gsd --db .sandbox/demo.db capture'
                                    # type "ignore me", Esc
gsd --db .sandbox/demo.db inbox     # unchanged
```

- [x] Agent verification before review: build the real binary, run the
      demo command list against a fresh temporary database, capture
      the verbatim output into the deck, and pass local `make check`.

## Chunk 2 — Capture contract

Human outcome: capture is a finished surface — misuse fails loudly
with guidance, a failed write explains itself inline, the chrome
degrades correctly without color, and the whole flow is proven by an
automated terminal-driven e2e.

Implementation:

- [x] Guards in `RunE` before `withTaskApplication`: `--json` refused
      with a `usageError` (config precedent); stdin and stdout checked
      separately through the generalized
      `presentationDependencies.isTerminal` seam; the non-TTY message
      names `gsd add` as the recovery path.
- [x] Error state: a failed `Add` renders the application error inline
      with the red accent; any key dismisses; the command returns the
      error so the root adapter emits the standard stderr line and
      exit 1.
- [x] In-flight submission lifecycle: run `Add` with a child context;
      Esc and Ctrl+C request cancellation, and the program waits for
      `Add` to return before quitting so command cleanup never closes
      the database under a running write. A canceled `Add` exits 0, an
      `Add` that wins the cancellation race keeps the normal success
      path, and any other failure enters the error state.
- [x] Color chain wired end to end: `--color` and `NO_COLOR` select
      the plain style set; the surface stays functional uncolored.
- [x] e2e tmux harness: private tmux server per test, popup-sized
      session, `send-keys`, exit-status collection; fails loudly when
      tmux is absent.
- [x] `.github/workflows/check.yml`: install tmux before `make check`.
- [x] README: tmux documented as a development/e2e prerequisite.

Verification (primary owners: model tests for the error state, `cmd`
tests for guards, tmux e2e for cross-invocation persistence):

- [x] Model: a failing fake `Add` renders the error inline; any key
      quits carrying the error; the styled and plain error renders
      both contain the message.
- [x] Model: blocked fake `Add` calls observe Esc/Ctrl+C cancellation;
      the program waits for the command result, maps context
      cancellation to a clean exit, preserves a success that wins the
      race, and never starts a duplicate `Add`.
- [x] `cmd`: `--json` and non-TTY invocations exit 2 with zero factory
      opens; messages match; help with an unopenable `--db` path still
      exits 0.
- [x] e2e (tmux, inside `make check`): a typed title plus Enter exits
      0 and a separate invocation's `inbox --json` shows the task with
      only the title set; Esc exits 0 and adds no row; the capture
      lands in the database selected by `--db` and by `GSD_DB`.
- [x] e2e (plain subprocess): `--json` and piped non-TTY invocations
      are usage errors, exit 2, nothing written.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/10-chunk-2.html`), exact
commands:

```sh
tmux display-popup -w 64 -h 4 -E 'env GSD_DB=.sandbox/demo-env.db gsd capture'
                                    # type "Buy stamps", Enter
gsd --db .sandbox/demo-env.db inbox # the env-selected database has it
gsd capture --json                  # usage error, exit 2
echo hi | gsd capture               # non-TTY: usage error, exit 2
tmux display-popup -w 64 -h 4 -E 'env NO_COLOR=1 gsd capture'
                                    # uncolored chrome, still works
chmod 400 .sandbox/demo-env.db
tmux display-popup -w 64 -h 4 -E 'env GSD_DB=.sandbox/demo-env.db gsd capture'
                                    # Enter: write fails inline; any
                                    # key: exit 1
```

- [x] Agent verification before review: build the real binary, run the
      demo command list against a fresh temporary database, capture
      the verbatim output into the deck, and pass local `make check`.

## Agent-verified end-to-end workflow

Run against the real built binary after both chunks merge
(`PROCESS.md` step 4); the equivalent durable coverage lives in `e2e/`
inside `make check`. Drive a popup-sized tmux pane with `send-keys`
against a fresh temporary database:

1. `make check` — includes the model, guard, and tmux e2e coverage.
2. Launch `gsd capture`, type a title, Enter: the program exits 0 with
   no output, and `gsd inbox --json` in a new invocation shows the
   task with only the title set.
3. Esc path: launch, type, Esc; exit 0, no row added.
4. Config chain: the capture lands in the database selected by `--db`
   and by `GSD_DB`.
5. Guards: `--json` and non-TTY invocation are usage errors, exit 2;
   `--help` with an unopenable `--db` path exits 0 without touching
   the database.
6. Report the clean transcript.

## Consolidation checklist (`PROCESS.md`)

- [x] `COMMANDS.md` documents `capture` as a shipped command — the
      ratified defaults (blank no-op, non-TTY usage error, inline
      write error with exit 1), Esc/Ctrl+C cancel, alt-screen, the
      chrome and color behavior, the `-w 64 -h 4` recommended popup,
      and tmux as the e2e prerequisite — and the TUI (post-v1) section
      is reconciled to point at it.
- [x] `DIVERGENCES.md` stays empty.
- [ ] Standard exit workflow plus `MILESTONE_10.md`'s exit criteria.
