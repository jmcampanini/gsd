# Issue 80 — Gate TUI background detection on color capability: root implementation plan

[Issue #80](https://github.com/jmcampanini/gsd/issues/80) is
authoritative for this fix's outcome and acceptance boundary;
[`plans/PROCESS.md`](plans/PROCESS.md) governs workflow and review.
This plan is temporary and is retired when the issue lands. One scope
correction settled at the plan gate: the issue predates the navigator
(#90), whose `gsd tui` carries the identical flaw at its own
background-query call site, so this fix owns both interactive paths —
the issue's "audit every background-query call site" requirement now
finds two.

## Progress

- [ ] Chunk 1 — Ask only terminals that can answer

## Settled design

Plan-gate interview, 2026-08-16.

- **The bug, precisely.** Bubble Tea executes the OSC 11 terminal
  interrogation unconditionally when a model returns
  `tea.RequestBackgroundColor` — it never consults the color
  profile — so the gate must live in this codebase. Both TUI entry
  points gate only on `Color != ColorDisabled`
  (`internal/tui/capture.go`, `internal/tui/navigator/model.go`), so
  a `ColorDetected` session whose effective profile is `ASCII` or
  `NoTTY` still interrogates the terminal. Ordinary command
  rendering already gates correctly in `presentation.output`:
  `profile >= colorprofile.ANSI && terminal`, with profile
  resolution injectable through `presentationDependencies`.
- **The gate is computed in `cmd`, before the program starts.** The
  capture and tui commands resolve the effective profile through the
  same injectable resolution ordinary rendering uses
  (`detectProfile` over the scrubbed environment), and pass resolved
  facts — not the color-mode decision — into the TUI layer.
- **`ProgramOptions` carries resolved facts.** `Color ColorMode` is
  replaced by the resolved `Profile colorprofile.Profile` plus the
  terminal-destination flag. `NewProgram` always pins
  `tea.WithColorProfile` to that profile, so the renderer and the
  gate cannot disagree. The color-mode decision enum becomes
  cmd-internal (renamed to avoid the existing `colorMode` flag-value
  type); `resolveColor` and the `--color` flag are untouched. The
  type is internal and fully updated in this change — not a breaking
  change.
- **One shared predicate.** A single capability predicate
  (profile ≥ ANSI and terminal destination) is used by
  `presentation.output` and both TUI runners, so the already-correct
  path and the fixed paths cannot drift. Models derive their
  existing `colorEnabled` bool from it; an ASCII-profile session now
  takes the existing plain-styling path.
- **At most one query, deterministic fallback.** The request stays
  in `Init`, which runs once per program execution. The
  `ThemeForBackground(true)` dark default remains the fallback when
  detection is skipped, unanswered, or fails — unchanged.
- **Forced color stays destination-aware.** Both commands already
  refuse non-terminal output with a usage error before any program
  or database opens; `--color always` into a pipe therefore never
  interrogates anything. Preserved unchanged.
- **Verification depth.** No new pseudo-terminal infrastructure. The
  background request is fire-and-forget in Bubble Tea — no code path
  of ours blocks on the answer — so the issue's respond/no-response
  PTY matrix has no gsd-owned behavior to pin. The capability matrix
  is owned by predicate/model/cmd tests with injected fakes; the
  existing tmux e2e suite keeps owning real-binary terminal
  restoration and the write-nothing rejection paths.
- **Plan shape.** Single chunk (one shared mechanism across both
  call sites); no consolidation checklist — background detection is
  not documented canonical behavior, so no `COMMANDS.md`,
  `OVERVIEW.md`, or `SCHEMA.md` reconciliation applies.

## Chunk 1 — Ask only terminals that can answer

Human outcome: `gsd capture` and `gsd tui` interrogate the terminal
background only when output is a terminal whose effective profile
can render ANSI color. `NO_COLOR`, `TERM=dumb`, `--color never`, and
ASCII-profile sessions render the plain UI with no interrogation;
redirected and piped invocations are refused exactly as today;
color-capable sessions behave identically to before.

Implementation:

- [x] `internal/tui/program.go`: `ProgramOptions` replaces
      `Color ColorMode` with resolved
      `Profile colorprofile.Profile` and a terminal-destination
      flag; `NewProgram` always applies
      `tea.WithColorProfile(options.Profile)`; the shared capability
      predicate lives here (suggested: a `ProgramOptions` method
      plus the bare function `cmd` reuses).
- [x] `internal/tui/capture.go`: `RunCapture` derives `colorEnabled`
      from the predicate; model, submission, and `runCapture`
      orchestration untouched.
- [x] `internal/tui/navigator/model.go`: `Run` derives
      `colorEnabled` the same way.
- [x] `cmd/presentation.go`: extend the resolution so TUI commands
      obtain profile, terminal flag, and scrubbed environment in one
      pass (e.g. fold profile into `colorResolution`);
      `presentation.output` routes its background-detection branch
      through the shared predicate; the decision enum
      (`ColorDisabled`/`ColorDetected`/`ColorForced`) relocates into
      `cmd` since `tui` no longer consumes it.
- [x] `cmd/capture.go`, `cmd/tui.go`: guards unchanged; build
      `ProgramOptions` from the resolved profile, terminal flag, and
      environment.
- [ ] Call-site audit recorded in the PR: exactly three
      background-detection sites — `presentation.output`
      (already correct, behavior preserved), capture `Init`,
      navigator `Init` — and no other
      `RequestBackgroundColor`/`HasDarkBackground` callers.

Verification (primary owners: predicate and model tests in
`internal/tui`; command wiring tests in `cmd`):

- [x] Predicate: full matrix — `NoTTY`, `ASCII`, `ANSI`, `ANSI256`,
      `TrueColor`, each with terminal and non-terminal
      destination — exactly ANSI-and-above on a terminal passes.
- [x] Model tests: capture and navigator `Init` return the
      background request exactly when the derived capability holds,
      and nothing ever issues a second request (adapt
      `TestCaptureInitRequestsBackgroundOnlyWithColor` and the
      navigator equivalent).
- [x] `cmd/capture_test.go`: extend
      `TestCaptureCommandPassesRuntimeDependenciesAndColorCapability` with
      an injected `detectProfile`: detected `TrueColor`/`ANSI256`/
      `ANSI` reach the runner query-capable; detected `ASCII` and
      `NoTTY` reach it incapable; `NO_COLOR`, `TERM=dumb`, and
      `--color never` are incapable; `--color always` on a terminal
      is capable; the environment stays scrubbed. `--color always`
      into a pipe keeps exiting 2 before the factory or runner is
      touched (existing rejection test owns it).
- [x] `cmd/tui_test.go`: mirror the same matrix in
      `TestTUICommandPassesApplicationsRuntimeOptionsAndLocation`.
- [x] Ordinary rendering preserved:
      `TestPresentationQueriesBackgroundOnceOnlyForStyledTerminalStdout`
      stays green through the shared predicate.
- [x] Existing e2e suite unchanged and green: tmux capture workflow
      (terminal restoration, clean pane, exit codes) and the
      noninteractive rejection tests (nothing written on refusal).
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/issue-80.html`), exact
commands:

```sh
gsd --db .sandbox/issue80.db capture
    # color terminal: themed input band; type a task; Enter
gsd --db .sandbox/issue80.db inbox
NO_COLOR=1 gsd --db .sandbox/issue80.db capture
    # plain UI, terminal-default cursor; Esc
TERM=dumb gsd --db .sandbox/issue80.db capture
    # plain UI; Esc
gsd --db .sandbox/issue80.db capture > .sandbox/issue80-redirect.txt
    # exit 2 refusal; the file stays empty
gsd --db .sandbox/issue80.db --color always capture | cat
    # exit 2 refusal — forced color into a pipe never interrogates
gsd --db .sandbox/issue80.db tui
    # themed navigator; q
NO_COLOR=1 gsd --db .sandbox/issue80.db tui
    # plain navigator; q
```

- [x] Agent verification before review: build the real binary, drive
      the command list in tmux against a fresh temporary database,
      capture the frames into the deck, and pass local `make check`.

## Agent-verified end-to-end workflow

Run tmux-driven against the real built binary and a fresh temporary
database once the chunk is complete; report the clean transcript.

1. In a color-capable tmux pane, `capture` renders the themed input
   band, adds a task on Enter, restores the pane clean, and exits 0;
   `inbox --json` shows exactly that task.
2. Re-run `capture` under `NO_COLOR=1` and under `TERM=dumb`: the
   pane frame contains no ANSI styling, Esc exits 0 with the pane
   restored, and no new row appears.
3. Redirected output and `--color always` piped through `cat` both
   exit 2 with the usage message on stderr, write nothing to stdout
   or the redirect target (no ANSI, OSC, or transient frames), and
   create no database.
4. `gsd tui` repeats the themed, plain, and refusal modes.
5. `make check` passes, covering the predicate/model/cmd matrices
   that pin exactly which profiles query and that none queries more
   than once.

The absence of the OSC 11 write itself is not observable in a tmux
pane capture; that assertion is owned by the model and command test
suites in step 5. If a byte-level terminal transcript is wanted
beyond that, verify manually with a PTY recorder.
