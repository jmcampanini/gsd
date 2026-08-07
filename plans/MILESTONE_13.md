# Milestone 13 — Input grammar

Data mode: **live**. Depends on: Milestone 12.

Written light; re-review at plan gate.

## Capability

Full parity, structurally: the CLI grammar arrives inside the TUI, and
capture grows syntax. One grammar-execution engine serves the `:`
command line, the edit/tag mnemonics, and capture's richer modes —
parity is shared code, not discipline.

## Scope

- **`:` opens a command line** accepting the CLI grammar verbatim,
  minus the binary name (`:projects add "Kitchen reno" --area 3`). It
  calls the same parser and core; results and errors render through
  Milestone 12's feedback surfaces.
- **`e` edit and `t` tag** arrive as mnemonics that prefill the command
  line for the selected row — verbs the single-key layer couldn't
  express without a grammar.
- **Inline capture syntax** (**proposed**: tag, date, and project
  tokens on the title) shared between `gsd capture` and the TUI's `a`
  quick add.
- **Capture runner mode**: the popup accepts the CLI grammar and
  executes it, riding the `:` engine.
- Internal ordering of the capture-syntax and command-line chunks is
  settled at plan gate.

## Chunks

1. **Grammar engine and `:`** — in-process execution of the CLI grammar
   against the same parser and services; result and error rendering.
2. **Edit and tag mnemonics** — `e`/`t` prefill on the selection.
3. **Inline capture syntax** — the token grammar, in `capture` and `a`.
4. **Capture runner mode** — the popup executes the grammar.

## Agent-verified end-to-end workflow

Tmux-driven against the real binary and a seeded temporary database:
run representative commands through `:` (add, edit, tag, cascade,
error cases) and verify equivalence with the CLI's `--json` output for
the same operations; capture with inline syntax lands the parsed
fields; runner mode round-trips a full command.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md` documents the capture syntax and runner mode.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003; TUI-applicable standards re-checked
at plan gate.
