# Milestone 11 — Boards

Data mode: **live**. Depends on: Milestone 9 (live baseline);
independent of Milestone 10's TUI substrate.

Settled by the 2026-08-07 boards interview (see `MILESTONES.md`
§ Decisions and history). Spellings originally marked (**proposed**)
were settled at the 2026-08-07 plan gate — root `PLAN.md` records the
decisions; this file reflects the settled surface. The governing
plan-gate decision: boards are areas-shaped (note, manual position,
`edit`-based mutation, the singular/plural noun split), not
tag-shaped.

## Capability

Boards make the pipeline a first-class concept: a **board** is a
user-defined pipeline of named, ordered **stages** (a software board
might run research → planning → doing → review; a life board something
else — gsd ships no defaults). A project is on at most one board and
occupies exactly one stage; moving it is a deliberate act.
The board is gsd's strategic lens — where the tactical views answer
"what should I do right now?", `gsd board show` answers "where is
everything, and what deserves to move next?"

Two task-level features make stages felt in daily work without giving
tasks a board position of their own: a task can **defer until a
stage** (hidden from `available` until its project reaches or passes
that stage), and a task can be marked as **promoting** (completing it
moves its project to the next stage, unlocking the next phase's
deferred tasks in the same stroke).

## Scope

- **Vocabulary**, used consistently everywhere: *board* (the entity;
  "pipeline" is prose only), *stage* (a named, ordered step of a
  board; the stored value), *column* (a stage as rendered in a board
  view — presentation only), *move* (the CLI verb, any direction),
  *promote* (movement to the next stage specifically — a concept, not
  a second verb).
- **Board model.** Boards are global — orthogonal to areas — and
  name-addressed: `NOCASE`-unique names, stored spelling displayed.
  Structurally they are areas-shaped: a note, a manual `position`
  ordering `boards list`, and `board edit --title/--note` (no rename
  verb). Each board owns its ordered stages; stage names are
  `NOCASE`-unique within their board. Stages order by `position` with
  the standard placement grammar; stage administration after creation
  ships in this milestone (`stages add`, `stage rename`,
  `stage reorder`, `stage delete`). Deleting a board or a stage with
  projects on it is `conflict`-restricted, like every occupied
  container.
- **Membership.** A project is on at most one board — membership is
  optional, and it is containment: `--board NAME | --no-board` on
  `projects add` and `project edit`, composing with `--area` (boards
  are orthogonal to areas). Entry always lands in the board's first
  stage — there is no named-stage entry; mid-pipeline entry composes
  `edit --board` with `move`. Switching boards re-enters the new
  board's first stage; `--no-board` clears the project's stage.
  Membership changes inherit the containment guards (resolved
  project, archived governing area → `conflict`). Projects order
  within a stage by `stage_position` with the placement grammar.
- **Movement** is a dedicated transition verb in the `done`/`cancel`
  family — `gsd project move ID STAGE [placement]` — any direction,
  no sequence enforcement, never gated on tasks. Bare cross-stage
  moves append; an explicit placement positions the project in the
  destination; a same-stage move with a placement is the within-column
  reorder. Resolved projects retain their hidden stage positions and
  remain valid placement references; `reorder` stays area-axis only.
- **Stage is orthogonal to status.** The board shows open projects
  only; resolving a project removes it from the board and the logbook
  keeps owning finished work. The last stage means "in final phase",
  never "done" — completion keeps its one spelling.
- **Defer until stage.** A task may name a stage of its project's
  board (`--defer-stage STAGE | --no-defer-stage`); it is hidden from
  `available` until the project's stage is at or past that stage.
  Composes with date defer — both must clear. Renders like deferred
  tasks today, and `list --deferred` covers both defer axes. On
  re-parenting away, `--no-board`, a board switch, or `stage delete`,
  the stage defer clears, with narration.
- **Promotes marker.** A task may be marked as promoting
  (`--promotes | --no-promotes`). Completing it transactionally moves
  its project one stage forward, narrated in the same envelope like
  the existing cascades. Promotion is one-way (reopening never
  demotes), relative (+1 from wherever the project sits), a reported
  no-op at the last stage, and inert when the project is on no board.
  The marker is opt-in per task and visible in listings as a faint
  `↑`.
- **`board show`** renders the strategic view: stages in order,
  open projects in position order under each, with derived task
  progress per project (done/total, cancelled excluded from both
  counts); `--json` emits the complete envelope.
- **Schema, as a pre-users clean break**: the board schema folds
  directly into `0001_baseline.sql` — no `0002` migration — with the
  contract-snapshot test updated to match. New `boards` and `stages`
  tables; `projects` gains its stage reference; `tasks` gain the
  defer-stage reference and the promotes marker. The stored stage is
  the schema's first genuine state column — justified because a stage
  is a position in a user-defined order, not a lifecycle flag; the
  promotes marker is declared intent, not an event. Existing live
  databases predate any users and are recreated by hand — documented,
  deliberate, once.
- **Boundaries per `AGENTS.md`**: services own semantic validation
  (unknown board/stage is `not_found`, occupied deletes are
  `conflict`, promotion composes in the service transaction); cmd
  stays adapter-thin; JSON and errors ride the shared writers.

### Command

```text
gsd boards add NAME --stage NAME [--stage NAME ...] [--note TEXT|-]
gsd boards list
gsd board show NAME
gsd board edit NAME [--title TEXT] [--note TEXT|-]
gsd board reorder NAME (--after NAME | --before NAME | --first | --last)
gsd board delete NAME
gsd stages add BOARD NAME [--first | --last | --after S | --before S]
gsd stage rename BOARD OLD NEW
gsd stage reorder BOARD NAME (--after S | --before S | --first | --last)
gsd stage delete BOARD NAME
gsd projects add "TITLE" ... [--board NAME]
gsd project edit ID [--board NAME | --no-board] ...
gsd project move ID STAGE [--first | --last | --after M | --before M]
gsd add|edit ... --defer-stage STAGE | --no-defer-stage
gsd add|edit ... --promotes | --no-promotes
```

Settled at the 2026-08-07 plan gate under the grammar rule recorded
in root `PLAN.md`: plural noun = collection operations, singular
noun = one entity, its argument being the entity's identity (ID or
NAME).

## Chunks

1. **Board model and administration** — baseline fold-in, `boards`
   and `stages` tables, `boards add`/`boards list`, board and stage
   administration, restricted deletes, `board show` over an empty
   board.
2. **Membership and movement** — the project stage reference,
   `--board`/`--no-board`, `move` with placement, within-stage
   ordering, `board show` with grouped projects and progress counts.
3. **Stage-aware tasks** — `--defer-stage` composed into `available`,
   the promotes marker, and transactional promotion with narration.

## Carried from Milestone 10

The Milestone 10 foundation review's findings were dispositioned and
paid on the milestone branch; nothing was scheduled, so there is no
chunk 0 work. The TUI-substrate promotion triggers carry in
`MILESTONE_12.md`; the remaining deliberately deferred items carry
forward here with their revisit triggers:

- **Truncation idiom consolidation** — `ansi.Truncate` with an `…`
  tail has one production call site (the capture error footer). On the
  second truncate-with-ellipsis call site — a `board show` column is a
  likely candidate — promote an `Ellipsize` helper into
  `internal/text` beside the escaping helper.
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

### The strategic view

```text
$ gsd boards add software --stage research --stage planning \
    --stage doing --stage review
+ Board: software (research → planning → doing → review)
$ gsd project edit 12 --board software
~ Edited: ◆ 12 gsd boards milestone → software/research
$ gsd project move 12 doing
~ Moved: ◆ 12 gsd boards milestone → doing
$ gsd board show software
software  research → planning → doing → review
  research  (empty)
  planning  ◆ 14  homelab backups        2/6
  doing     ◆ 12  gsd boards milestone   5/8
            ◆ 9   blog rewrite           1/3
  review    (empty)
```

### One stroke, whole phase transition

```text
$ gsd edit 87 --promotes            # "Write the plan" is the capstone
$ gsd done 87
✓ Done: Write the plan
~ Promoted: ◆ 12 gsd boards milestone → doing
$ gsd available                     # tasks deferred until doing appear
```

## Agent-verified end-to-end workflow

Against the real built binary and a temporary database:

1. Create a board; `boards list --json` and `board show --json` carry
   the stages in defined order; stage administration reorders and
   renames; deleting an occupied stage and deleting an occupied board
   are `conflict`, exit `1`.
2. Put a project on a board with `project edit --board` (enters the
   first stage) and another with `projects add --board`; move it
   forward and backward; reorder within a stage via same-stage `move`
   with a placement; `board show` groups and orders correctly in
   human and JSON output; `--no-board` clears the stage.
3. Defer until stage: a task deferred until a later stage is absent
   from `available`, appears the moment the project reaches or passes
   that stage, and date defer still gates independently.
4. Promotion: completing a marked task advances the project one stage
   and narrates it; reopening the task does not demote; completing a
   marked task at the last stage reports the no-op; the whole write is
   atomic.
5. Status orthogonality: `project done` removes the project from
   `board show` and it appears in `logbook`; reopen returns it to its
   stage.
6. Guards: unknown board and stage names are `not_found`; a
   `--defer-stage` naming a stage outside the project's board is
   refused; help and parse failures never open the database.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md` documents the boards family, `move`,
      `--defer-stage`, and the promotes marker.
- [ ] `SCHEMA.md` documents `boards`, `stages`, the appended columns,
      and the recorded baseline fold-in decision.
- [ ] `OVERVIEW.md` gains the boards concept and vocabulary.
- [ ] The typed-transition-spec deferral carried above is paid or
      explicitly re-deferred with a new trigger — this milestone adds
      the transitions that trip it.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-001/003.
