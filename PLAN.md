# Milestone 6 — Root Implementation Plan

Config, color modes, and the styled human-output layer. This is the
authoritative implementation and verification plan for the active milestone
under [`plans/PROCESS.md`](plans/PROCESS.md);
[`plans/MILESTONE_6.md`](plans/MILESTONE_6.md) owns the outcomes and
acceptance boundary. Both artifacts are retired at consolidation.

## Progress

- [x] Chunk 0 — Milestone 5 consolidation
- [x] Chunk 1 — Config loader integration
- [x] Chunk 2 — `gsd config` report
- [ ] Chunk 3 — Color resolution and styled human output

## Settled contract deltas

Decisions from the 2026-08-02 planning interview and the three blind pairwise
styling tests, already reconciled into `plans/MILESTONE_6.md`,
`plans/COMMANDS.md`, and `plans/MILESTONES.md`:

- **Color is not configuration.** The only v1 config key is `db_path`. The
  color chain is strictly CLI-OUTPUT-001: explicit `--color=auto|always|never`
  flag > nonempty `NO_COLOR` > destination-aware auto, evaluated per stream;
  `TERM=dumb` disables in auto; `FORCE_COLOR`/`CLICOLOR*` are never consulted.
  `GSD_COLOR` and the `color` TOML key do not exist.
- **Structure is mode-independent; SGR is mode-dependent.** Headers, glyphs,
  and layout appear in every color mode; `never`/`NO_COLOR` strip only ANSI
  styling. Supported `--json` output never contains ANSI under any mode.
- **The config report is TOML-only.** Its default output is valid,
  redirectable TOML; `db_path` is rendered as its absolute effective runtime
  location so relative env/flag values round-trip from any snapshot location.
  `--provenance` keeps it valid TOML by adding inline comments normalized to
  `default`, `file: PATH`, `env: GSD_DB`, or `flag: --db`. Despite the
  inherited global `--json` flag, `gsd config --json` is a usage error (exit
  2); TOML is this command's machine-readable
  format.
- **`db_path` is intentionally reportable and non-sensitive.** Chunk 2 adds
  no redaction code or hooks. The reporting contract must be revisited before
  any future sensitive key is added.
- **The visual system** (blind-test verdicts, rounds 1–3) is specified in the
  style guide below.

## Architecture decisions

- **`internal/config`** (new package) owns the config struct, defaults
  (including XDG resolution), go-config-loader wiring in
  defaults → TOML file → env → flags order, provenance data, and the coded
  error for config that cannot load (`invalid_argument`, exit 1). Only an
  absent discovered file is optional; an invalid discovered file stops the
  chain before env and flags, so higher-precedence values cannot patch over
  it. This strict behavior remains while go-config-loader issue #13 evaluates
  the alternative. `store.ResolvePath`'s default-path logic migrates here;
  stores receive a final path. Relative file values are anchored to the
  config file's directory; relative env and flag values keep their current
  working-directory behavior.
- **Config struct**: one field, two tags —
  `DBPath string` with `toml:"db_path"`, `config:"db"`, `help:"..."` — so
  `GSD_DB`/`--db` keep their exact current spellings. Empty `GSD_DB` and
  explicit `--db ""` remain key-specific legacy fallbacks rather than a
  general policy for future settings. Flag registration goes through
  `pflagloader` so the flag surface has a single source of truth; registration
  failures are static programmer errors and panic during root construction.
- **cmd owns all color and styling** (presentation, per `AGENTS.md`), routed
  through the existing shared writers. Services and stores are untouched by
  chunk 3.
- **Color resolution** is a pure function per stream:
  `(flagValue, noColorEnv, isTerminal, TERM) → mode`. Invalid `--color`
  values fail flag parsing as usage errors (exit 2) via a `pflag.Value`
  implementation. Profile selection: `never` → no styling; `always` →
  TrueColor; `auto` → colorprofile detection on that stream with a scrubbed
  environment (`NO_COLOR`/`FORCE_COLOR`/`CLICOLOR*` removed — our chain owns
  NO_COLOR semantics and the standard forbids the others).
- **Background detection**: `lipgloss.HasDarkBackground(stdin, stdout)` is
  queried once, only when stdout styling is active and stdout is a terminal;
  Frappé (dark) is the default when the query fails or output is piped under
  `--color=always`. `lipgloss.LightDark` selects accent values.
- **Help, version, and argument parsing** continue to open neither the
  database nor the config file; `gsd config` loads config but never opens
  the database.
- **Config errors retain chained library context**, including the
  `configloader:` prefix. Error codes are stable; message wording is not.
- **JSON envelopes are byte-identical to Milestone 5.** Only human output
  changes.

## Style guide (blind-test verdicts)

Accent palette — the only hues in the system, accents-only (plain text and
faint hierarchy never carry palette colors):

| Accent | Latte (light) | Frappé (dark) | Applied to |
|--------|---------------|---------------|------------|
| green  | `#40a02b`     | `#a6d189`     | `✓`, `+`, `done` status words |
| red    | `#d20f39`     | `#e78284`     | `✗`, `−`, `cancelled` status words, urgent due dates |

Grammar: **faint** = metadata (IDs, kinds, counts, timestamps, headers,
non-urgent dates) · **bold** = urgency (due today or overdue, combined with
red) · **glyphs** = records and events · **headers** = whisper · identity
markers (`#`, `•`, `◆`, `└`) stay monochrome.

Per surface:

- **Collections** (`inbox`, `available`, `list`, `projects`, `areas`,
  `logbook`): aligned tables with lowercase faint column headers rendered
  only when rows exist (empty collections still print nothing), no rules or
  borders, right-aligned faint IDs, plain titles, faint status/kind words
  (`done` tinted green, `cancelled` red), faint dates with urgent due tokens
  bold red. Current 2-space gutter geometry is preserved.
- **`tags`**: headerless `#name  count` rows — faint `#`, plain name, faint
  count.
- **`show`**: status-glyph headline (`• 12  Book dentist appointment` — glyph
  reflects entity status), then indented lowercase faint field labels with
  plain values; empty fields kept; note keeps line breaks; tags render as
  `#a #b` with faint `#`.
- **Mutation lines**: verb-class glyph prefix on today's payloads —
  `+` add (tasks, projects, areas, tags), `−` delete, `✓` done,
  `✗` cancel, `#` tag/untag, `•` other mutations (edit, reopen, rename,
  archive, unarchive, move). Glyphs colored by outcome class (green/red);
  neutral glyphs monochrome; IDs faint; tag names after `#` plain.
- **Recursive delete narration**: glyph-prefixed verb line, plain count line,
  `└`-tree children with faint ids.
- **stderr**: unstyled in v1; the resolver still evaluates stderr
  independently so styling it later is a one-line change.

## Chunk 0 — Milestone 5 consolidation

Review-only (no new human-facing capability): applies the structural
decisions from the Milestone 5 foundation review before Config work begins.
Behavior-preserving except the tag-ordering schema delta (`ORDER BY g.title
COLLATE NOCASE`, `user_version` → `9006`). No demo.

- [x] Root/core store split: executor-bound core types; root type owns the
      handle and each verb's visible atomicity decision; `database == nil`
      mode flag, re-entry preambles, and `Tasks.List`'s wrap predicate die;
      `applyTransition` retains a single action switch.
- [x] Read-then-delete: the six `WITH snapshot AS MATERIALIZED` deletes
      become tag-enriched reads plus plain deletes inside the existing
      immediate-transaction machinery.
- [x] Service-owned tag resolution on the read path: `list --tag` resolves in
      the service; the store filter loses its resolution half.
- [x] Sibling alignment: tag service flows and cmd tagging factories
      byte-parallel modulo entity nouns; `<noun>TaggingMutation` /
      `new<Noun>TaggingCommand` naming.
- [x] Tag delete drops the redundant leading `Find`; sequence assertion
      narrows to `CountUsage` before `Delete`.
- [x] Shared `tagUsageCountExpression`; `collectRows[T]` replaces the five
      rows→slice collectors.
- [x] `domain.TagNames` becomes the `Tags` field type everywhere (nil
      marshals `[]`).
- [x] Alphabetical tag arrays: views and `tagJSONExpression` move to
      `ORDER BY g.title COLLATE NOCASE`; `user_version` → `9006`;
      order-asserting tests updated.
- [x] Verification: full `make check`; schema convergence audit re-run
      (dev database byte-comparable to `SCHEMA.md`).

## Chunk 1 — Config loader integration

Human outcome: the database location becomes a setting — a discovered TOML
file drives `db_path` with full precedence, and an explicit `--config` fails
loud.

- [x] `internal/config`: struct, defaults (XDG data-home db default moved
      from `store.ResolvePath`), loader order defaults → discovered/explicit
      TOML → env → flags, `LoadReport` provenance retained for chunk 2.
- [x] Root command: `--config PATH` persistent flag; `--db` registered via
      `pflagloader`; the application factory resolves the db path through the
      loaded config; help/version/parse-error paths load nothing.
- [x] Explicit `--config` that is missing, unreadable, or invalid TOML →
      coded `invalid_argument`, exit 1; an existing discovered file that
      cannot load does the same, even when env or flags provide a valid path;
      only an absent discovered file is silently fine. A file-provided empty
      `db_path` is `invalid_argument` (CLI-CONFIG-002; current
      go-config-loader behavior documented while issue #13 evaluates both
      strict and patch-over contracts).
- [x] Test owners: `internal/config` tests own the precedence matrix and
      failure modes with real temp files/env/flag values (cheapest faithful
      layer — the loader is real, no store involved); cmd tests own flag
      plumbing and exit mapping with a factory spy proving help/version open
      nothing; e2e proves existing `--db`/`GSD_DB` behavior unchanged and
      file-driven `db_path` end to end (db file appears at the configured
      path).
- [x] Human proof (demo `.sandbox/demos/milestone-6-chunk-1.html`):
      `mkdir -p $XDG_CONFIG_HOME/gsd` + write `config.toml` with `db_path`;
      `gsd add "First"` then `ls` the configured location;
      `GSD_DB=... gsd add "Second"` showing env override;
      `gsd add "Third" --db ...` showing flag override;
      `gsd inbox --config /nonexistent.toml` failing exit 1.
- [x] Agent verification: `make check` green; precedence matrix exercised
      via the built binary.

## Chunk 2 — `gsd config` report

Human outcome: you can always see what config is in effect and why.

- [x] `gsd config`: valid, redirectable TOML of the effective config via
      configreporter; `db_path` is rendered as its absolute effective runtime
      location so relative env/flag values remain stable when reloaded from
      another directory; `--provenance` adds an inline source comment
      normalized to `default`, `file: PATH`, `env: GSD_DB`, or `flag: --db`;
      the command never opens the database. `gsd config --json` is a usage
      error (exit 2) despite the inherited global flag; TOML is its
      machine-readable format.
- [x] No redaction code or hooks: `db_path` is intentionally reportable and
      non-sensitive. Revisit the reporting contract before adding any future
      sensitive key.
- [x] Test owners: cmd tests own TOML/provenance rendering, `--json`
      rejection, stream routing, and exit mapping (config report is
      presentation over `internal/config` data); `internal/config` tests own
      provenance correctness and make the file-precedence row's expected
      path/source declarative while extending that matrix; e2e owns the
      round-trip (`gsd config` output, including a relative env value, fed
      back via `--config` from another directory yields an identical effective
      config).
- [x] Human proof (demo `milestone-6-chunk-2.html`): `gsd config`;
      `GSD_DB=... gsd config --provenance`; `gsd config > snap.toml && gsd
      config --config snap.toml`; `gsd config --json` rejected with exit 2.
- [x] Agent verification: `make check` green; round-trip, fail-loud
      `--config`, and `gsd config --json` rejection exercised via the built
      binary.

## Chunk 3 — Color resolution and styled human output

Human outcome: gsd's human surface becomes the settled visual system —
readable hierarchy, glyph vocabulary, and adaptive red/green accents — while
redirected and JSON output stay byte-clean.

- [ ] Color resolver in cmd: pure per-stream function
      (flag > nonempty `NO_COLOR` > auto), `pflag.Value` for
      `--color=auto|always|never` (bare or invalid value → usage, exit 2),
      `TERM=dumb` and non-terminal disable in auto, scrubbed-env
      colorprofile detection, `always` → TrueColor.
- [ ] Background query once via `HasDarkBackground` when stdout styling is
      active on a terminal; Frappé default otherwise; `LightDark` accent
      selection.
- [ ] Shared writers restyled to the style guide: quiet-headed collection
      tables (headers only when rows exist), `#` tags surface, glyph-headline
      `show`, verb-class glyph mutation lines, `└`-tree delete narration.
      JSON writers untouched.
- [ ] Test owners: resolver unit tests in cmd own the full mode matrix
      including both TTY branches (terminal-ness injected — the only faithful
      owner, since subprocesses have no pty); writer tests in cmd own
      rendered structure and ANSI presence/absence per mode for
      representative fixtures; e2e owns piped reality: default pipe clean,
      `--color=always` pipe carries ANSI, `--color=always` beats `NO_COLOR`,
      `NO_COLOR` clean, supported `--json` clean under `--color=always`,
      `TERM=dumb` clean.
- [ ] Human proof (demo `milestone-6-chunk-3.html`, ANSI converted to HTML
      spans by the demo generator): `gsd available`, `gsd list --status all`,
      `gsd show N`, `gsd logbook`, `gsd tags`, a done/cancel/delete
      narration sequence; `gsd inbox > inbox.txt && cat -v inbox.txt`;
      `NO_COLOR=1 gsd inbox`; `gsd inbox --color=always | less -R`.
- [ ] Agent verification: `make check` green; full color matrix via the
      built binary.

## Agent-verified end-to-end workflow

Run against the real built binary after all chunks merge; the durable
equivalent lives in `e2e/` inside `make check`.

1. `db_path` precedence matrix: default only; file; file+env; file+env+flag —
   each layer wins over the previous, verified by `gsd config --provenance`
   and by where the database file actually appears.
2. `gsd config` output round-trips: feed its TOML back via `--config`;
   effective config identical. `gsd config --json` is rejected as usage
   (exit 2).
3. Explicit `--config /nonexistent` fails loud, exit 1; absent discovered
   file is silently fine.
4. Color matrix (piped): default redirect clean; `--color=always` emits ANSI
   into a pipe; `--color=always` beats nonempty `NO_COLOR`; `NO_COLOR`
   yields clean output; `TERM=dumb` clean in auto; supported `--json` output
   carries no ANSI under `--color=always`. (TTY-only auto branches are owned
   by resolver unit tests; no pty in e2e.)
5. Structure is mode-independent: `--color=never` output contains headers
   and glyphs, differing from `--color=always` only in ANSI bytes.
6. `make check` proves all pre-existing e2e workflows still pass with config
   in the loading path.
