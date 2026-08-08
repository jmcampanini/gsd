# Milestone 11 — Boards: root implementation plan

[`plans/MILESTONE_11.md`](plans/MILESTONE_11.md) is authoritative for
this milestone's outcomes and acceptance boundary;
[`plans/PROCESS.md`](plans/PROCESS.md) governs workflow, review, and
consolidation. This plan is temporary and is retired at consolidation.

## Progress

- [x] Chunk 1 — Boards exist and bend
- [ ] Chunk 2 — Projects on the board
- [ ] Chunk 3 — Stage-aware tasks

There is no chunk 0: the Milestone 10 foundation review scheduled
nothing; the deferred items carry forward in `MILESTONE_11.md` with
their revisit triggers.

## Settled design

Plan-gate interview, 2026-08-07. The governing decision: **boards are
areas-shaped, not tag-shaped** — they carry a note and a manual
position, mutate through `edit` (no `rename` verb), and use the
singular/plural noun split. Every proposed spelling in the milestone
file was settled here; `MILESTONE_11.md`'s command block and user
stories were reconciled to match.

- **Grammar rule** (recorded in `COMMANDS.md` at consolidation):
  plural noun = collection operations, singular noun = one entity; the
  singular's argument is the entity's identity — ID for ID-addressed
  entities, NAME for name-addressed ones. Boards are the first
  name-addressed entity with singular-noun operations; tags remain a
  special case only because bare `tag` is an attach verb.
- **Command surface** (final):

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
  gsd add|edit ... [--defer-stage STAGE | --no-defer-stage]
                   [--promotes | --no-promotes]
  ```

- **Membership is containment.** `--board` rides `projects add` and
  `project edit` exactly like `--area`; the two compose (boards are
  orthogonal to areas); `--board` with `--no-board` is the standard
  set-plus-clear usage error. Entry always lands in the board's first
  stage — there is no named-stage entry; mid-pipeline entry is `edit
  --board` then `move`. Switching boards re-enters the new board's
  first stage. `--no-board`, a board switch, re-parenting a task away,
  and `stage delete` all clear affected stage defers inside the same
  transaction, narrated as `├`/`└ Cleared stage defer:` children.
  Membership changes inherit the containment guards: on a resolved
  project or under an archived governing area they are `conflict` with
  reopen/unarchive-first guidance.
- **`move` owns the whole board axis.** Any direction, no sequence
  enforcement, never gated on tasks; a transition in the
  `done`/`cancel` family, guarded like one. Bare cross-stage moves
  append to the destination column; an explicit placement positions
  the project in that destination; a same-stage move with a placement
  is the within-column reorder; a bare same-stage move is a no-op.
  Placement references are project IDs in the same stage, including
  resolved projects that retain hidden positions. `reorder` stays
  area-axis only.
- **Naming.** Board and stage names use title validation (nonblank
  UTF-8, stored unchanged). Boards are `NOCASE`-unique globally;
  stages `NOCASE`-unique within their board. Lookups resolve
  case-insensitively and display the stored spelling; conflicts name
  the existing stored spelling.
- **Schema** (baseline fold-in — the board schema lands directly in
  `0001_baseline.sql`, no `0002`; the contract-snapshot test is
  updated per chunk; the pre-users live database is recreated by hand,
  once):
  - `boards`: id, `title` UNIQUE `NOCASE`, `note`, `position`,
    timestamps — STRICT, like `areas` plus name uniqueness.
  - `stages`: id, `board_id` FK `ON DELETE CASCADE` (stages are parts
    of the board's definition; occupancy is service-guarded first and
    `projects.stage_id` RESTRICT is the backstop), `title`,
    `position`, timestamps; UNIQUE (`board_id`, `title` `NOCASE`).
  - `projects` appends `stage_id` (FK stages RESTRICT) and
    `stage_position`, with `CHECK ((stage_id IS NULL) =
    (stage_position IS NULL))` — a project sits in two independent
    orders (area siblings, stage column), so the board axis gets its
    own position column, same ordinal-only semantics.
  - `tasks` appends `defer_stage_id` (FK stages RESTRICT — never
    tripped: services clear references first in the deleting
    transaction) and `promotes` INTEGER NOT NULL DEFAULT 0 `CHECK
    (promotes IN (0, 1))` — the schema's first genuine boolean,
    justified because the marker is declared intent, not an event.
  - The `available` view gains the stage-defer clause: visible when
    `defer_stage_id` is NULL or the project's stage position has
    reached the deferred stage's position. Date defer still gates
    independently — both must clear.
  - Indexes: `idx_stages_board`, `idx_projects_stage`,
    `idx_tasks_defer_stage`.
- **Semantics.**
  - Creation requires at least one `--stage`; `stage delete` may empty
    a board (only occupancy blocks); `--board` onto a stageless board
    is `conflict`.
  - Deleting an occupied board or stage is `conflict`; deleting an
    empty board deletes its stages with it.
  - `board show` renders open projects only, grouped by stage in
    position order, with derived progress `done/total` where cancelled
    tasks are excluded from both counts. Resolving a project removes
    it from the board; reopen returns it to its stage and position.
  - Defer-until-stage requires the task's project to be on a board;
    the stage must belong to that board (`invalid_argument`
    otherwise; unknown names are `not_found`). Comparison is
    reaches-or-passes by stage position, so moving backward re-hides.
    Stage-deferred tasks render like date-deferred tasks, and
    `list --deferred` covers both defer axes.
  - `--promotes` is settable on any task and inert without a boarded
    project (completion is then silent). Promotion is one-way
    (reopen never demotes), relative (+1 from wherever the project
    sits), transactional inside the task service's `Done`, narrated
    `~ Promoted:` in the same envelope, and a reported no-op at the
    last stage.
- **Envelopes.** Entity JSON stays bare table rows: projects gain
  `stage_id`/`stage_position`, tasks gain `defer_stage_id` and
  `promotes` (rendered as JSON booleans); names resolve through the
  board envelopes. `boards list` is an array of board rows each
  carrying an ordered `stages` array of stage rows; `board show` is
  `{"board":{...},"stages":[{...stage row...,"projects":[project row
  + "progress":{"done":N,"total":M}]}]}`; `board delete` returns
  `{"board":{...},"stages":[...]}`. A board-flag `project edit`
  returns `{"project":{...},"cleared_defers":[task rows]}`; a
  promoting completion returns
  `{"task":{...},"promoted_project":{...}}` with `promoted_project`
  null on the last-stage no-op. Errors ride the standard coded
  envelope; exit codes stay 0/1/2.
- **Presentation.** Creation echo `+ Board: software (research →
  planning → doing → review)`; `boards list` rows `NAME  stage →
  stage → …` in position order; `board show` per the user story with
  `(empty)` columns and no truncation (the carried `Ellipsize`
  trigger does not fire); board-changing edit lines carry the result
  (`~ Edited: ◆ 12 … → software/research`); `~ Moved:` and
  `~ Promoted:` mutation lines; the promotes marker is a faint `↑`
  beside the title in listings and a `promotes` row in `show`;
  project `show` gains a board row (`software/research`); task `show`
  gains defer-stage and promotes rows.
- **Architecture** (per `AGENTS.md`): new `internal/board` package
  owning board/stage administration and the `board show` read model;
  membership and `move` live in the project service; defer-stage
  validation and promotion composition live in the task service's
  `Done` transaction; concrete SQL in `internal/store` behind narrow
  interfaces; cmd stays adapter-thin with lazy factory opening, so
  help and parse failures never touch the database.
- **Carried-deferral dispositions.** The typed-transition-spec
  deferral is explicitly re-deferred: `move` and promotion never
  enter the task store's transition switch (verified against
  `internal/store/tasks.go`), so the refactor would be unrelated to
  this diff. New trigger: pay when a new action case enters the task
  store's transition switch. The truncation-idiom trigger does not
  fire (`board show` does not truncate).

## Chunk 1 — Boards exist and bend

Human outcome: create a board with its stages, view it empty, and
reshape it — add, rename, reorder, and delete stages; edit, reorder,
and delete boards — with occupied deletes refused once projects
arrive (proven in chunk 2).

Implementation:

- [x] `0001_baseline.sql`: `boards` and `stages` tables with
      uniqueness, FK, and index definitions above; contract-snapshot
      test updated to the new end state.
- [x] `internal/store/boards.go`: board and stage CRUD, name
      resolution (`NOCASE`, stored spelling), position renumbering
      via the shared reorder helpers, and ordered stage snapshots for
      the board-delete result. Occupancy queries land with project
      stage storage in chunk 2.
- [x] `internal/board`: entities plus service — `boards add`
      (≥1 stage, transactional), `list`, `show` (empty board renders
      all stages), `edit`, `reorder` (name-referenced placement),
      `delete`; `stages add` (placement, default `--last`),
      `stage rename`, `stage reorder`, `stage delete`; semantic
      validation (`not_found` unknown names and `conflict` duplicates).
      Occupied-delete conflicts become reachable and land in chunk 2.
- [x] `cmd/boards.go`: the ten commands wired through an injected
      board application factory; shared writers for JSON and errors;
      usage errors for missing/duplicate placement flags, matching
      the reorder precedent.

Verification (primary owners: store tests on a real temp database for
schema and query semantics; service tests with store fakes for
validation; cmd tests for envelopes and exits):

- [x] Store: uniqueness (`NOCASE` global for boards, per-board for
      stages), stage cascade on board delete, contiguous renumber on
      stage/board reorder, snapshot test green.
- [x] Service: creation requires a stage; duplicate names conflict
      naming stored spelling; case-insensitive lookup; unknown board
      or stage `not_found`; empty-board `show` carries every stage.
- [x] cmd: envelope shapes (`boards list` rows with `stages` arrays,
      `board show` envelope, delete envelope), exit codes, and zero
      factory opens for `--help`.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/11-chunk-1.html`), exact
commands:

```sh
gsd --db .sandbox/demo1.db boards add software --stage research \
    --stage planning --stage doing --stage review
gsd --db .sandbox/demo1.db boards list
gsd --db .sandbox/demo1.db board show software
gsd --db .sandbox/demo1.db stages add software intake --first
gsd --db .sandbox/demo1.db stage rename software intake triage
gsd --db .sandbox/demo1.db stage reorder software triage --after research
gsd --db .sandbox/demo1.db stage delete software triage
gsd --db .sandbox/demo1.db board edit software --note "the shipping pipeline"
gsd --db .sandbox/demo1.db boards add life --stage someday --stage active
gsd --db .sandbox/demo1.db board reorder life --first
gsd --db .sandbox/demo1.db boards list
gsd --db .sandbox/demo1.db board delete life
gsd --db .sandbox/demo1.db board show software
```

- [x] Agent verification before review: build the real binary, run
      the demo command list against a fresh temporary database,
      capture the verbatim output into the deck, and pass local
      `make check`.

## Chunk 2 — Projects on the board

Human outcome: put projects on a board, move them through and within
stages, and read the strategic view — stages in order, open projects
grouped with progress counts — with status orthogonality and occupied
deletes proven live.

Implementation:

- [x] `0001_baseline.sql`: `projects.stage_id`/`stage_position` with
      the pair CHECK and index; snapshot test updated.
- [x] `internal/store`: project stage assignment (append on entry),
      clearing, within-stage splice reorder, board and stage occupancy
      queries, and the grouped `board show` read with per-project
      done/total counts (cancelled excluded).
- [x] Project service: `--board`/`--no-board` orchestration on add
      and edit — first-stage entry, board switch re-entry, stage
      clearing, containment guards, stageless-board `conflict` —
      returning the edit envelope (`cleared_defers` empty until
      chunk 3 populates it).
- [x] Project service: `move` — name resolution against the project's
      board, bare cross-stage append, explicit destination placement,
      same-stage placement reorder, same-stage bare no-op, transition
      guards.
- [x] Board service: occupied board/stage deletes now `conflict`.
- [x] `cmd`: `--board` on `projects add`, `--board`/`--no-board` on
      `project edit` (set-plus-clear usage error), `project move`
      with placement flags, board-result edit lines, `~ Moved:`
      lines, full `board show` rendering.

Verification (primary owners: store tests for the column pair and
grouped read; service tests for orchestration and guards; cmd tests
for envelopes and rendering):

- [x] Store: pair CHECK enforced; entry appends to the column;
      explicit cross-stage placement splices into the destination;
      within-stage renumber touches `updated_at` on the moved row
      only; progress counts exclude cancelled; `board show` read
      groups and orders by `stage_position, id`.
- [x] Service: unknown board/stage `not_found`; stageless board
      `conflict`; guards on resolved/archived; switch re-enters first
      stage; bare same-stage move is a no-op; placement references
      outside the stage are `invalid_argument`.
- [x] cmd: move echo is the project row; `board show` envelope and
      human layout; `--board --no-board` usage error, exit 2.
- [x] `make check` green.

Human proof (chunk demo `.sandbox/demos/11-chunk-2.html`), exact
commands:

```sh
gsd --db .sandbox/demo2.db boards add software --stage research \
    --stage planning --stage doing --stage review
gsd --db .sandbox/demo2.db projects add "gsd boards milestone"
gsd --db .sandbox/demo2.db projects add "blog rewrite"
gsd --db .sandbox/demo2.db projects add "homelab backups" --board software
gsd --db .sandbox/demo2.db project edit 1 --board software
gsd --db .sandbox/demo2.db project edit 2 --board software
gsd --db .sandbox/demo2.db project move 1 doing
gsd --db .sandbox/demo2.db project move 2 doing --first
gsd --db .sandbox/demo2.db project move 1 doing --before 2
gsd --db .sandbox/demo2.db project move 3 planning
gsd --db .sandbox/demo2.db board show software
gsd --db .sandbox/demo2.db project done 2
gsd --db .sandbox/demo2.db board show software
gsd --db .sandbox/demo2.db logbook
gsd --db .sandbox/demo2.db project reopen 2
gsd --db .sandbox/demo2.db board show software
gsd --db .sandbox/demo2.db board delete software
gsd --db .sandbox/demo2.db project edit 2 --no-board
gsd --db .sandbox/demo2.db board show software
```

- [x] Agent verification before review: build the real binary, run
      the demo command list against a fresh temporary database,
      capture the verbatim output into the deck, and pass local
      `make check`.

## Chunk 3 — Stage-aware tasks

Human outcome: defer a task until a stage and watch it surface the
moment the project arrives; mark a capstone task `↑` and complete it
to promote the whole project — one stroke, whole phase transition.

Implementation:

- [ ] `0001_baseline.sql`: `tasks.defer_stage_id` and
      `tasks.promotes` with CHECK and index; the `available` view
      gains the stage-defer clause; snapshot test updated.
- [ ] Task service: `--defer-stage`/`--no-defer-stage` validation
      (project on a board, stage of that board) on add and edit;
      `--promotes`/`--no-promotes` on add and edit.
- [ ] Task service `Done`: promotion composed in the transaction —
      resolve the project's next stage, advance with column append,
      narrate; reported no-op at the last stage; reopen leaves the
      stage untouched.
- [ ] Clearing wiring: `--no-board`/board switch (project service),
      re-parenting a task away (task service), and `stage delete`
      (board service) clear affected stage defers in the same
      transaction and report them (`cleared_defers`).
- [ ] `cmd`: the four task flags, `↑` in listings and `show`,
      `~ Promoted:` narration, cleared-defer trees, promotion and
      cleared-defers JSON envelopes.
- [ ] `e2e/boards_test.go`: the six-step workflow below as durable
      subprocess coverage inside `make check`.

Verification (primary owners: store tests for view semantics; service
tests for validation, composition, and atomicity via the transaction
seam; cmd tests for envelopes and glyphs; e2e for cross-invocation
persistence):

- [ ] Store: `available` hides a stage-deferred task until the
      project's stage reaches or passes the target; moving backward
      re-hides; date and stage defer gate independently; both-clear
      visibility.
- [ ] Service: defer-stage refused off-board (`invalid_argument`) and
      for unknown stages (`not_found`); promotion advances exactly
      one stage, appends to the destination column, no-ops with a
      report at the last stage, is inert off-board, and rolls back
      atomically when the stage write fails (fake-injected error);
      reopen never demotes; each clearing path reports its cleared
      set.
- [ ] cmd: `--defer-stage` with `--no-defer-stage` (and promotes
      pair) usage errors; promotion envelope; `↑` rendering; deferred
      rendering matches date-deferred tasks; `list --deferred`
      includes stage-deferred rows.
- [ ] e2e: the six-step workflow passes against the real binary.
- [ ] `make check` green.

Human proof (chunk demo `.sandbox/demos/11-chunk-3.html`), exact
commands:

```sh
gsd --db .sandbox/demo3.db boards add software --stage research \
    --stage planning --stage doing --stage review
gsd --db .sandbox/demo3.db projects add "gsd boards milestone" --board software
gsd --db .sandbox/demo3.db project move 1 doing
gsd --db .sandbox/demo3.db add "Land chunk 3" --project 1
gsd --db .sandbox/demo3.db add "Draft announcement" --project 1 --defer-stage review
gsd --db .sandbox/demo3.db available
gsd --db .sandbox/demo3.db edit 1 --promotes
gsd --db .sandbox/demo3.db list --project 1
gsd --db .sandbox/demo3.db done 1
gsd --db .sandbox/demo3.db available
gsd --db .sandbox/demo3.db board show software
gsd --db .sandbox/demo3.db add "Ship it" --project 1 --promotes
gsd --db .sandbox/demo3.db done 3
gsd --db .sandbox/demo3.db reopen 1
gsd --db .sandbox/demo3.db board show software
gsd --db .sandbox/demo3.db add "Retro notes" --project 1 --defer-stage shipping
gsd --db .sandbox/demo3.db project edit 1 --no-board
```

- [ ] Agent verification before review: build the real binary, run
      the demo command list against a fresh temporary database,
      capture the verbatim output into the deck, and pass local
      `make check`.

## Agent-verified end-to-end workflow

Run against the real built binary and a fresh temporary database
after all chunks merge (`PROCESS.md` step 4); the equivalent durable
coverage lives in `e2e/` inside `make check`:

1. Create a board; `boards list --json` and `board show --json` carry
   the stages in defined order; `stage rename` and `stage reorder`
   reshape it; deleting an occupied stage and deleting an occupied
   board are `conflict`, exit 1.
2. Put a project on a board with `project edit --board` (enters the
   first stage) and another with `projects add --board`; `move` it
   forward and backward; reorder within a stage via same-stage `move`
   with a placement; `board show` groups and orders correctly in
   human and JSON output; `--no-board` clears the stage.
3. Defer until stage: a task deferred until a later stage is absent
   from `available`, appears the moment the project reaches or passes
   that stage, and date defer still gates independently.
4. Promotion: completing a `--promotes` task advances the project one
   stage and narrates it; reopening the task does not demote;
   completing a marked task at the last stage reports the no-op; the
   whole write is atomic.
5. Status orthogonality: `project done` removes the project from
   `board show` and it appears in `logbook`; reopen returns it to its
   stage.
6. Guards: unknown board and stage names are `not_found`; a
   `--defer-stage` naming a stage outside the project's board is
   refused; help and parse failures never open the database.

Report the clean transcript.

## Consolidation checklist (`PROCESS.md`)

- [ ] `COMMANDS.md`: the boards and stages families, `--board`
      membership flags, `move`, `--defer-stage`, `--promotes`, the
      name-addressed grammar rule, and the board/stage semantics and
      envelopes above.
- [ ] `SCHEMA.md`: `boards`, `stages`, the appended project and task
      columns, the `available` view change, the recorded baseline
      fold-in decision, and a note on `promotes` as the deliberate
      first boolean column.
- [ ] `OVERVIEW.md`: the boards concept and vocabulary (board, stage,
      column, move, promote).
- [ ] The typed-transition-spec re-deferral recorded with its new
      trigger; remaining carried items re-homed per `PROCESS.md`.
- [ ] `DIVERGENCES.md` stays empty.
- [ ] Standard exit workflow plus `MILESTONE_11.md`'s exit criteria.
