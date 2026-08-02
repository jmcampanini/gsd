# Milestone 4 Implementation Plan

This is the authoritative implementation plan for the outcomes in
[`plans/MILESTONE_4.md`](plans/MILESTONE_4.md). The milestone's capability,
command surface, semantics, user stories, and acceptance workflow define what
must ship. Its implementation chunks are suggestions; the chunks below define
how the work will be delivered.

Data remains throwaway. Milestone 4 starts from the consolidated Milestone 3
projects baseline and changes the development schema revision from `9003` to
`9004`. An older or nonempty development database must fail with the existing
conflict and deletion guidance; there is no migration.

## Delivery shape

Milestone 4 is three sequential vertical slices. This is the chunk progress
checklist; check a chunk only after its implementation and verification items
below are complete:

- [x] **Chunk 1 — Areas exist** — a human can create, list, inspect, and edit
      areas, with the complete revision-`9004` schema landed and every new
      view predicate proven by store tests.
- [x] **Chunk 2 — Containment** — a human can place projects and loose tasks
      in areas, re-parent across containers, filter by area, and see the
      schema-enforced mutual exclusion and governing-area output.
- [ ] **Chunk 3 — Archive and delete** — a human can archive and unarchive an
      area, watch `available` respect the governing area, list the archived
      partition, and delete areas with RESTRICT protection and an explicit
      recursive opt-in guarded by retired-history semantics.

Each chunk crosses the application, SQLite, command, output, and test
boundaries needed for its outcome and ends with built-binary human proof.
Each chunk starts only after the previous one is reviewed and squash-merged
into the milestone branch.

Expected review size, including tests:

- Chunk 1: approximately 700–900 changed lines across schema, the new area
  application, commands, and view tests.
- Chunk 2: approximately 900–1,200 changed lines across task and project
  containment, scoped positions, enrichment output, and commands.
- Chunk 3: approximately 1,000–1,300 changed lines for archive semantics,
  the archived-area guard, listing partition, and recursive deletion.

These are review-size guides, not targets. If a chunk grows enough to obscure
its outcome, revise this plan before splitting it.

## Settled behavior

### IDs and positions

Areas join the `AUTOINCREMENT` rule: separate integer ID namespace, no
automatic reuse after committed deletion. Area positions order the single
global list of areas; an archived area keeps its position and returns to it
on unarchive. Task append positions become scoped by the full containment
predicate (`project_id IS ? AND area_id IS ?`); project append positions
become scoped by their sibling group (`area_id IS ?`). Insertion append
remains `COALESCE(MAX(position), -1) + 1` over the scoped predicate.

### Containment and mutual exclusion

A task belongs to exactly one of a project, an area, or nothing (inbox),
enforced by `CHECK (project_id IS NULL OR area_id IS NULL)`. Supplying
`--project` and `--area` together on task `add` or `edit` is
`invalid_argument`, validated by the task service and backed by the CHECK.
`gsd list --project N --area M` is the same contradiction and fails
identically. Within one container pair, `--area`/`--no-area` and
`--project`/`--no-project` are each mutually exclusive command grammar
(usage, exit 2); a set flag paired with the other container's clear
(`--project N --no-area`) is a redundant but consistent spelling and is
accepted, as is `--no-project --no-area` (move to inbox).

Re-parenting appends to the end of the destination container and clears the
other containment column in the same atomic statement; re-stating the
current container is a no-op and does not move the entity. Moving a task out
of or into a resolved project remains `conflict`; a blocked move names every
blocker. Referencing a nonexistent area is `not_found` everywhere an area ID
can appear: task `add --area`, `edit --area`, `list --area`, `projects add
--area`, `projects list --area`, `project edit --area`, and every singular
`area` verb. `gsd list --area N` filters by direct containment — the area's
loose tasks — not by governing inheritance.

Projects gain the same shape: `projects add --area N` creates into the area,
`project edit N --area M | --no-area` re-parents with append-on-move and
no-op re-statement, and `projects list --area N` filters by containment. A
resolved project's own fields, including its area, remain editable; only
archived source or destination areas block its moves.

### Area listing partition

`areas list` hides archived areas by default. `--archived` lists only
archived ones; `--all` lists both. The two flags are mutually exclusive
command grammar. All slices order by `position`, then `id`.

### The archived-area guard

An archived area is retired history, mirroring the resolved-project guard
through the governing area (own, or inherited through the project). These
operations return `conflict` with unarchive-the-area-first guidance:

- creating a task or project into an archived area;
- re-parenting a task or project into or out of an archived area; and
- task or project `done`, `cancel`, or `reopen` when the entity's governing
  area is archived.

Content edits (title, note, dates) of governed entities, edits of the
archived area itself, and deletes stay allowed — deletes never block on
state, only on RESTRICT. A move blocked by more than one guard (resolved
projects, archived areas) names every blocker in one conflict. Archiving
never mutates contents: no cascade, and unarchive restores visibility with
every position intact. Archiving an already-archived area and unarchiving an
active one are `conflict` through the existing guard-and-classify pattern.

### Deletion

`area delete N` on an area containing any project or loose task is
`conflict` (RESTRICT). `area delete N --recursive` deletes the contained
projects' tasks, the contained projects, the loose tasks, and the area in
one transaction, regardless of archived or project status, and returns the
envelope `{"area":{...},"deleted_projects":[...],"deleted_tasks":[...]}` —
`deleted_tasks` holding both loose and project-contained tasks. Arrays are
ordered `position, id` and may be `[]`. Task deletes never block.

### Output

JSON remains one compact, newline-terminated value. Task and project rows
gain nullable `area_id` (the table-row contract); area rows are their table
row. `available` and `inbox` emit the full view row — the task columns plus
`project_title`, `governing_area_id`, and `governing_area_title` — so every
task-shaped view row has identical columns; `logbook` entries gain
`governing_area_id` and `governing_area_title`. The `tags` column waits for
Milestone 5.

Human output: `areas list` is a headerless aligned table of id, title, and
an `archived` marker cell (blank when active); `area show` is a field/value
table of the area row. Task and project `show` gain an `Area` row containing
the entity's own area ID; human collection rows gain no area column in
Milestone 4, and the governing-area enrichment is not surfaced in human
rows. `area archive`/`area unarchive` print concise action-prefixed mutation
lines (`Archived: area 1  Home`). Recursive deletion narrates the mutation
line, then `Deleted N project(s):` and `Deleted N task(s):` sections, each
omitted when empty. Empty collections print nothing; no color or ANSI
styling before Milestone 6.

Bare `gsd areas` and bare `gsd area` are usage errors with exit code 2 and
must not open the database.

## Implementation boundaries

### Area application

New `internal/area` package following the project package's shape: an `Area`
value (table-row JSON), `AddFields`, `EditFields`, `ListOptions` carrying the
validated listing slice (active, archived, or all; active default), and the
narration envelope `Deletion` (`area` + `deleted_projects` +
`deleted_tasks`), referencing `project.Project` and `task.Task`. The
consumer-defined `area.Store` interface carries persistence primitives:
`Add`, `Find`, `List`, `Edit`, `Archive`, `Unarchive`, `Delete`,
`DeleteProjects`, `DeleteTasks`, plus
`WithinTransaction(context.Context, func(Store) error) error`. The service
owns ID/title/note validation, slice parsing, timestamping, and the
recursive-delete transaction scope and mutation sequence.

### Task application

`task.Task` gains `AreaID *int64` (`json:"area_id"`). `AddFields` gains
`AreaID`; `EditFields` gains an `Area AreaChange` value mirroring
`ProjectChange`. The service validates the set-and-clear contradiction,
positive area IDs, and the cross-container set-and-set contradiction
(`invalid_argument`); existence, archived-area, and resolved-project guards
are enforced at the store where the mutation is atomic. `ListOptions` gains
an optional area filter and rejects the project-and-area filter combination.

### Project application

`project.Project` gains `AreaID *int64` (`json:"area_id"`). `AddFields`
gains `AreaID`; `EditFields` gains an `Area AreaChange`. `ListOptions` gains
an optional area filter. Validation follows the task package's split.

### SQLite store

Chunk 1 establishes the complete Milestone 4 schema in one revision:

- `areas` table per `SCHEMA.md`, with `AUTOINCREMENT`;
- `projects.area_id` and `tasks.area_id`, both
  `REFERENCES areas(id) ON DELETE RESTRICT`;
- the containment `CHECK (project_id IS NULL OR area_id IS NULL)`;
- `idx_tasks_area` and `idx_projects_area` (`idx_tasks_project` shipped in
  Milestone 3);
- views rebuilt to their near-final form: `inbox` (both containers NULL,
  full enrichment block), `available` (archived-governing-area exclusion via
  `COALESCE(t.area_id, p.area_id)`, full enrichment block), `logbook`
  (governing-area columns on both branches); and
- `PRAGMA user_version = 9004`.

Landing the final schema once keeps revision `9004` truthful throughout the
milestone. View predicates whose CLI surface arrives later — the archived
exclusion and enrichment columns — are proven by store tests with raw-SQL
fixtures in Chunk 1, the pattern Milestone 3 used for `logbook`.

`store.Areas` joins the per-entity stores, sharing `store.DB` and the scan
helpers. Single-entity mutations remain single atomic statements whose
predicates enforce the guards, with classification queries producing
`not_found` or `conflict` after a miss. Containment predicates classify in
order: missing container → `not_found`; resolved project → `conflict`;
archived governing area → `conflict`; blocked moves gather every blocker
before reporting. Recursive area deletion is the milestone's one
multi-statement transaction: the area service owns its scope through
`WithinTransaction`; `store.Areas` implements the boundary with
`BEGIN IMMEDIATE`, a transaction-bound store, `COMMIT`, and rollback on any
failure, deleting project tasks, then projects, then loose tasks, then the
area, returning affected rows ordered `position, id`. Task-shaped view
queries scan into a view-row value embedding the task columns plus the
enrichment block. List SQL is assembled only from validated enums and IDs.

### Command and output adapters

`areas` and `area` parent commands return usage errors when invoked bare.
Cobra owns flag declaration, changed-flag detection, same-pair mutual
exclusion (`--area`/`--no-area`, `--archived`/`--all`), and construction of
application intent; the area service is wired through the existing injected
factory. Help, version, argument parsing, and flag conflicts must not open
the database. All success and failure rendering flows through the shared
writers. Unarchive-first and reopen-first recovery guidance naming flags or
command spellings is composed at the command adapter, not below it.

## Test ownership

Each behavior has one primary owner at the cheapest layer that proves the
likely regression. Every added test must state the regression or explicit
requirement it protects and why its layer is the cheapest faithful proof.

- **Store tests with real temporary SQLite databases** own revision-`9004`
  bootstrap, `areas` constraints and ID non-reuse, the containment CHECK,
  scoped append positions (proven with interleaved inserts across inbox,
  project, and area containers asserting in-container values), cross-container
  re-parent atomicity (set clears the other column), view predicates
  (inbox both-NULL exclusion, available archived-governing-area exclusion
  through both own and inherited paths, enrichment column correctness,
  logbook governing columns), guard predicates and their
  `not_found`/`conflict` classification (raw-SQL fixtures make archived
  areas before Chunk 3 makes them reachable), multi-blocker conflict
  gathering, archive/unarchive classification, RESTRICT classification, and
  recursive-delete atomicity including rollback on failure.
- **Service tests with store fakes** own semantic validation (IDs, titles,
  notes, listing-slice parsing, set-and-clear and set-and-set
  contradictions), edit-intent construction, error pass-through, and
  non-nil collection normalization. Fakes return canned values; they do not
  simulate transactions.
- **Command tests** own flag adaptation, same-pair mutual-exclusion and
  bare-noun usage errors with no database open, JSON envelope shapes (task
  and project `area_id`, area rows, the full view row, the `Deletion`
  envelope), human rendering (areas table with archived marker, `Area`
  rows, archive mutation lines, deletion narration including omitted empty
  sections), stream selection, and exit mapping. They do not restate store
  predicate matrices.
- **The Milestone 4 subprocess workflow in `e2e/`** owns confidence that the
  real binary persists area containment across invocations and wires
  archive, the guard, the partition, and recursive deletion together.
  Chunk 1 introduces it; Chunks 2 and 3 extend it to the complete milestone
  workflow.

## Chunk 1 — Areas exist

### Human outcome

A human can create areas, list them in position order, inspect and edit
them, and every Milestone 4 schema change is live under revision `9004`.

### Implementation

- [x] Land the complete revision-`9004` schema: `areas` table, `area_id`
      columns with RESTRICT and indexes, the containment CHECK, the three
      rebuilt views, and the version bump. Prove bootstrap, constraints,
      ID non-reuse, and all view predicates with real SQLite, including the
      archived exclusion and enrichment columns via raw-SQL fixtures ahead
      of their CLI surface.
- [x] Add `internal/area` (values, `Store` interface with `Add`, `Find`,
      `List`, `Edit`, service validation) and `store.Areas` with global
      append positions.
- [x] Add `areas add "TITLE" [--note TEXT|-]`, `areas list`, `area show N`,
      and `area edit N [--title TEXT] [--note TEXT|-]`; bare `gsd areas` and
      bare `gsd area` are usage errors, exit 2, without opening the
      database.
- [x] Render area rows: JSON table rows; human `areas list` id/title table
      and `area show` field/value table.
- [x] Introduce the Milestone 4 subprocess workflow: area CRUD persistence
      across invocations.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/4-chunk-1.html`):

- [x] `gsd areas add "Home"`, `gsd areas add "Health"`; `gsd areas list`
      shows both in creation order.
- [x] `gsd area show 1` renders the field/value table; `gsd area edit 1
      --note "Everything house"` persists and `gsd area show 1` reflects it.
- [x] `gsd areas add ""` fails `invalid_argument` with exit 1; `gsd area
      show 99` fails `not_found` with exit 1.
- [x] Bare `gsd areas` and bare `gsd area` are usage errors with exit 2.

## Chunk 2 — Containment

### Human outcome

A human can place projects and loose tasks in areas, re-parent tasks and
projects across containers with append-on-move, filter lists by area, see
the mutual exclusion defend itself, and see governing-area columns in view
output.

### Implementation

- [x] Carry `area_id` through the task value, JSON, scanner, and store round
      trips; add `--area` to task `add` and `list`; add mutually exclusive
      `--area`/`--no-area` to task `edit` with cross-container re-parent
      (set clears the other column atomically), append-on-move, and
      same-container no-op semantics; scope task append positions by the
      full containment predicate.
- [x] Validate the set-and-set contradiction in the task service
      (`invalid_argument` for `--project`/`--area` together on `add`,
      `edit`, and as `list` filters).
- [x] Carry `area_id` through the project value and store; add `--area` to
      `projects add` and `projects list` and `--area`/`--no-area` to
      `project edit` with append-on-move into the destination sibling group
      and no-op re-statement; scope project append positions by area.
- [x] Enforce membership guards at the store: nonexistent area →
      `not_found` on every path; moving a task out of or into a resolved
      project stays `conflict` and blocked moves name every blocker.
- [x] Emit the full view row from `available` and `inbox` (task columns
      plus `project_title`, `governing_area_id`, `governing_area_title`)
      and add the governing columns to `logbook` output.
- [x] Add the `Area` row to human task and project `show`; keep collection
      rows unchanged.
- [x] Extend the subprocess workflow: containment across invocations,
      mutual exclusion, re-parent append, and governing-area columns.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/4-chunk-2.html`):

- [x] `gsd areas add "Home"`; `gsd projects add "Kitchen reno" --area 1`;
      `gsd add "Change furnace filter" --area 1`; `gsd add "Get quotes"
      --project 1`; a loose `gsd add "Buy milk"`.
- [x] `gsd list --area 1` shows only the furnace filter (direct
      containment); `gsd projects list --area 1` shows Kitchen reno;
      `gsd inbox` shows only the loose task.
- [x] `gsd show 1` displays the `Area` row; `gsd project show 1` displays
      the `Area` row.
- [x] `gsd add "Impossible" --project 1 --area 1` fails `invalid_argument`
      with exit 1; `gsd edit 3 --area 1 --no-area` is a usage error with
      exit 2.
- [x] `gsd edit 2 --area 1` moves the project task to the area's task list
      end and clears its project; `gsd edit 2 --project 1` moves it back,
      appended last.
- [x] `gsd areas add "Work"`, `gsd project edit 1 --area 2` re-parents the
      project; `gsd projects list --area 2` shows it appended;
      `gsd project edit 1 --no-area` returns it to the standalone group.
- [x] `gsd add "X" --area 99` fails `not_found` with exit 1.

## Chunk 3 — Archive and delete

### Human outcome

A human can archive an area and watch everything it governs leave
`available` while `list` and `show` still see it, unarchive with every
position intact, list the archived partition, watch the retired-history
guard defend mutations, and delete areas with RESTRICT protection and a
narrated recursive opt-in.

### Implementation

- [ ] Extend `area.Store` with `Archive`, `Unarchive`, `Delete`,
      `DeleteProjects`, `DeleteTasks`, and `WithinTransaction`; have the
      area service build the `Deletion` envelope while owning the
      recursive-delete transaction scope and sequence. Implement the
      boundary in `store.Areas` with guard-and-classify conflicts,
      `position, id` ordering, and rollback proof.
- [ ] Add the archived-area guard to task and project mutation predicates
      through the governing area: creation into, re-parenting into or out
      of, and `done`/`cancel`/`reopen` under an archived governing area →
      `conflict`; multi-blocker moves gather resolved-project and
      archived-area blockers into one conflict; content edits and deletes
      stay allowed.
- [ ] Classify archive/unarchive: archiving an archived area and
      unarchiving an active one → `conflict`.
- [ ] Add `area archive N`, `area unarchive N`, and
      `area delete N [--recursive]`; add mutually exclusive `--archived`/
      `--all` to `areas list` with the partition semantics and the
      `archived` marker cell in human rows.
- [ ] Resolve `area.ParseListSlice`: use it in the Chunk 3 production flag
      path or delete it and its tests if the boolean flags map directly to
      listing-slice constants.
- [ ] Compose unarchive-first recovery guidance at the command adapter;
      render the deletion narration and JSON envelope.
- [ ] Extend the subprocess workflow to the complete milestone workflow:
      archive visibility by inheritance, the guard, the partition,
      unarchive restoration, RESTRICT, and recursive deletion.
- [ ] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/4-chunk-3.html`):

- [ ] Build Home with a loose task and a project holding two tasks, plus one
      inbox task; `gsd available` shows all four tasks.
- [ ] `gsd area archive 1` prints `Archived: area 1  Home`; `gsd available`
      shows only the inbox task; `gsd list --area 1` still sees the loose
      task; `gsd area show 1` still renders.
- [ ] `gsd areas list` prints nothing but the active areas; `gsd areas list
      --archived` shows only Home; `gsd areas list --all` shows every area
      with the archived marker on Home.
- [ ] `gsd add "Late idea" --area 1` fails `conflict` with unarchive-first
      guidance; `gsd done` on the project's task fails `conflict`;
      `gsd edit` on that task's title succeeds.
- [ ] `gsd area unarchive 1` restores all four tasks to `available` in their
      original order; `gsd area unarchive 1` again fails `conflict`.
- [ ] `gsd area delete 1` fails `conflict` (not empty); `gsd area delete 1
      --recursive` narrates the deleted project and every deleted task.
- [ ] `gsd areas add "Empty"` then `gsd area delete 2` succeeds without
      `--recursive`.

## Consolidation and exit

After every chunk is reviewed and squash-merged into the milestone branch:

- [ ] Reconcile the accepted semantics into the canonical specifications:
      the areas listing partition and `--all` grammar, the archived-area
      guard, area `not_found` references, the mutual-exclusion
      `invalid_argument` classification, and the recursive area deletion
      envelope into `plans/COMMANDS.md` and `plans/OVERVIEW.md`.
- [ ] Diff the shipped `available` view SQL against `SCHEMA.md`'s
      definition — the remaining delta must be exactly the `tags` column
      (the milestone's specific exit criterion) — and confirm the other
      views and tables match.
- [ ] Reconcile tests around stable observable contracts and codify lasting
      review findings in `AGENTS.md`, lint/build configuration, or the
      owning tests.
- [ ] Run the automated Milestone 4 subprocess workflow through
      `make check`.
- [ ] Build the real binary and retain a clean transcript of the workflow
      below.
- [ ] Have Javier demo the milestone user stories.
- [ ] Update the roadmap and links, then permanently delete
      `plans/MILESTONE_4.md` and root `PLAN.md` as required by
      `plans/PROCESS.md`.
- [ ] Rerun `make check`, require green final CI, review the consolidated
      milestone pull request, and squash-merge it into `main`.

## Agent-verified end-to-end workflow

Use the real built binary, a fresh temporary database under `.sandbox/`, and
JSON output except where human rendering is explicitly inspected.

- [ ] Build: area A with a loose task and a project holding two tasks; area
      B with one loose task; one inbox task. Assert each created row echoes
      its `area_id` or `project_id`.
- [ ] Assert `gsd available` returns all five tasks and each row's
      `governing_area_id` is correct — inherited for the project tasks, own
      for the loose tasks, `null` for the inbox task — with
      `project_title` and `governing_area_title` populated to match.
- [ ] `gsd area archive A`: assert `available` drops to the area-B task
      plus the inbox task, `gsd list --area A` still sees the loose task,
      and `gsd inbox` is unchanged.
- [ ] Assert the guard while A is archived: `gsd add --area A`,
      re-parenting a task into and out of A, `gsd done` on a project task
      governed by A, and `gsd project done` on its project all fail
      `conflict` with exit 1, while a title edit and a task delete under A
      succeed (recreate the deleted task before continuing).
- [ ] Assert the partition while A is archived: `gsd areas list` returns
      only B, `--archived` returns only A, `--all` returns both;
      `--archived --all` together is a usage error with exit 2.
- [ ] `gsd area unarchive A`: assert all five tasks return to `available`
      with positions intact, and `gsd area unarchive A` again fails
      `conflict`.
- [ ] Assert mutual exclusion: task `add` and `edit` with both containers,
      and `gsd list --project N --area M`, all fail `invalid_argument` with
      exit 1.
- [ ] Re-parent the project to area B; assert its tasks' `governing_area_id`
      follows and the project appended to the end of B's project list; assert
      re-stating its area does not move it.
- [ ] Delete a task inside the project and the loose task in area A; assert
      both succeed (task deletes never block).
- [ ] Assert `gsd area delete B` fails `conflict` (it holds the project and
      its loose task), and `gsd area delete A` — now empty — succeeds.
- [ ] `gsd area delete B --recursive`: assert one transaction returns the
      envelope with the project in `deleted_projects` and both the
      project's remaining task and B's loose task in `deleted_tasks`.
- [ ] Assert `gsd add --area 99` and `gsd list --area 99` fail `not_found`
      with exit 1, and bare `gsd areas` and bare `gsd area` are usage errors
      with exit code 2 and no database side effect.
- [ ] Inspect human output for the archive mutation line, the `areas list
      --all` archived marker, one guarded conflict with unarchive-first
      guidance, the `Area` row on task `show`, and one recursive deletion
      narration (no ANSI styling).
- [ ] Run `make check` and report the command transcript and final clean
      result.
