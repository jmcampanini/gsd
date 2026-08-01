# Milestone 3 Implementation Plan

This is the authoritative implementation plan for the outcomes in
[`plans/MILESTONE_3.md`](plans/MILESTONE_3.md). The milestone's capability,
command surface, semantics, user stories, and acceptance workflow define what
must ship. Its implementation chunks are suggestions; the chunks below define
how the work will be delivered.

Data remains throwaway. Milestone 3 starts from the consolidated Milestone 2
time baseline and changes the development schema revision from `9002` to
`9003`. An older or nonempty development database must fail with the existing
conflict and deletion guidance; there is no migration.

## Delivery shape

Milestone 3 is one review-only structural chunk followed by three sequential
vertical slices. This is the chunk progress checklist; check a chunk only
after its implementation and verification items below are complete:

- [x] **Chunk 0 — Structural refactor** — review-only: shared error package,
      the `Store` interface vocabulary, and per-entity store types, with no
      behavior change.
- [x] **Chunk 1 — Projects exist** — a human can create and inspect projects,
      place tasks in them, re-parent tasks, and see `inbox` and `available`
      respect containment.
- [x] **Chunk 2 — Lifecycle and cascade** — a human can complete, cancel,
      reopen, and delete projects, with the cascade narrating what it
      cancelled and deletion honoring RESTRICT.
- [ ] **Chunk 3 — Logbook** — a human can see finished tasks and projects
      interleaved, newest first, with kind markers.

Chunk 0 is the one permitted review-only chunk: it isolates a mechanical,
repo-wide refactor so every later diff reads as feature work. It is not
followed by another review-only chunk. Chunks 1–3 each cross the application,
SQLite, command, output, and test boundaries needed for their outcome and end
with built-binary human proof. Each chunk starts only after the previous one
is reviewed and squash-merged into the milestone branch.

Expected review size, including tests:

- Chunk 0: approximately 400–600 changed lines, almost entirely mechanical.
- Chunk 1: approximately 1,000–1,400 changed lines across schema, the new
  project application, task containment, and commands.
- Chunk 2: approximately 700–1,000 changed lines for lifecycle, the
  transactional cascade, deletion, and guards.
- Chunk 3: approximately 350–550 changed lines for the logbook read model,
  command, and rendering.

These are review-size guides, not targets. If a chunk grows enough to obscure
its outcome, revise this plan before splitting it.

## Settled behavior

### Containment and re-parenting

A task belongs to at most one project (`project_id` NULL = inbox until areas
arrive). `gsd add --project N` creates into the project; `gsd edit N
--project M | --no-project` owns re-parenting, and the two flags are mutually
exclusive command grammar. Re-parenting appends to the end of the destination
container; re-stating a task's current container is a no-op and does not move
it. Insertion order is `COALESCE(MAX(position), -1) + 1` scoped by the
containment predicate (`project_id IS ?`), never the whole table; the
predicate extends with `area_id` in Milestone 4.

Referencing a nonexistent project is `not_found` everywhere it can occur:
`add --project`, `edit --project`, and `list --project`.

### The resolved-project guard

A done or cancelled project is closed history. These operations return
`conflict` with reopen-the-project-first guidance:

- task `done`, `cancel`, or `reopen` when the task's project is resolved;
- `add --project` into a resolved project; and
- re-parenting a task into or out of a resolved project.

Content edits (title, note, due, defer) of tasks in resolved projects and
task deletion remain allowed. Deletes never block on state, only on RESTRICT.
When a task move is blocked because both its source and destination projects
are resolved, the conflict names both projects and tells the human to reopen
both rather than revealing one blocker per retry.

### Cascade, reopen, and deletion

`project done N` and `project cancel N` stamp the project's exit and cancel
its remaining open tasks in one transaction, using one shared timestamp from
a single clock sample. Already-resolved tasks are untouched. The response is
the narration payload: the project row plus the cancelled tasks ordered by
`position, id`. Resolving an already-resolved project is `conflict` through
the existing guard-and-classify pattern.

`project reopen N` clears the project's exit and nothing else: cascade is
never undone, and cancelled tasks stay cancelled until individually reopened.

`project delete N` on a project with tasks is `conflict` (RESTRICT).
`project delete N --recursive` deletes the tasks and the project in one
transaction and returns both for narration, regardless of project status.

### Logbook

The `logbook` view unions resolved tasks and projects with the Milestone 3
column set `kind, id, title, status, resolved_at, project_title`
(`resolved_at = COALESCE(done_at, cancelled_at)`; area and tags columns join
in Milestones 4–5). The view carries no ORDER BY; the CLI orders entries by
`resolved_at` descending, then project entries before task entries resolved
at the same instant, then `id` descending, so a cascade lists the project
above the tasks it cancelled.

### Output

JSON remains one compact, newline-terminated value. Task rows gain nullable
`project_id` (the table-row contract). Project rows are their table row.
Cascade responses are `{"project":{...},"cancelled_tasks":[...]}` and
recursive deletion mirrors it as `{"project":{...},"deleted_tasks":[...]}`;
both arrays may be `[]`. Logbook entries are the view's column set.

Human output: `projects list` is a headerless aligned table of id, title,
and status; `project show` is a field/value table; task `show` gains a
`Project` row containing the project ID. Task collection rows gain no
project column in Milestone 3, and the `project_title` view enrichment is
not yet surfaced in command output. Cascade narration prints the mutation
line, then a `Cancelled N open task(s):` section listing each task; the
section is omitted when nothing was cancelled. Recursive deletion narrates
each deleted task the same way. The human logbook prints kind, id, title,
status, and the local calendar day of `resolved_at`. Empty collections print
nothing; no color or ANSI styling before Milestone 6.

Bare `gsd projects` and bare `gsd project` are usage errors with exit code 2
and must not open the database.

## Implementation boundaries

### Structural refactor (Chunk 0)

Extract the stable error codes and constructors from `internal/task` into a
new `internal/apperr` package used by the task, store, and command layers. Use
the idiomatic package-qualified vocabulary `Code`, `NotFound`,
`InvalidArgument`, `Conflict`, `Internal`, `New`, and `CodeOf`. Rename the
persistence interface vocabulary from `Repository` to `Store`: services
depend on consumer-defined `Store` interfaces declared in their entity
packages. Split the concrete store into a `store.DB` type owning path
resolution, open, pragmas, bootstrap, and close, plus a per-entity
`store.Tasks` type holding only task SQL. Construct it explicitly with
`store.NewTasks(*store.DB)` so `store.DB` does not become an entity-store
factory. Update `AGENTS.md` to the services/stores vocabulary. No exported
behavior, SQL, or output changes.

### Project application

New `internal/project` package following the task package's shape: a
`Project` value (table-row JSON), `AddFields`, `EditFields`, `ListOptions`
with the standard status filter defaulting to open, an `Exit` enum, and the
narration envelopes `Resolution` (`project` + `cancelled_tasks`) and
`Deletion` (`project` + `deleted_tasks`), which reference `task.Task`. The
consumer-defined `project.Store` interface carries persistence primitives:
`Add`, `Find`, `List`, `Edit`, `Resolve`, `CancelOpenTasks`, `Reopen`,
`Delete`, and `DeleteTasks`, plus
`WithinTransaction(context.Context, func(Store) error) error`. The service
owns ID/title/note validation, status parsing, exit selection, timestamping,
and multi-statement orchestration. Single-statement use cases call one store
method; resolve and recursive delete enter `WithinTransaction` and compose
their project and task mutations through the transaction-bound `Store`.

### Task application

`task.Task` gains `ProjectID *int64` (`json:"project_id"`). `AddFields`
gains `ProjectID`; `EditFields` gains a `Project ProjectChange` value
mirroring `DateChange` so omitted, set, and cleared cannot collapse. The
service validates the internally contradictory set-and-clear combination and
positive project IDs; existence and openness are enforced at the store where
the mutation is atomic. `ListOptions` gains an optional project filter.

### SQLite store

Chunk 1 establishes the complete Milestone 3 schema in one revision:

- `projects` table per `SCHEMA.md` minus `area_id`;
- `tasks.project_id REFERENCES projects(id) ON DELETE RESTRICT` and
  `idx_tasks_project` (the containment CHECK waits for `area_id`);
- `inbox` view gains `project_id IS NULL`; `available` gains the
  project-open clause and `project_title` enrichment; the `logbook` view
  lands with its Milestone 3 columns; and
- `PRAGMA user_version = 9003`.

Landing the final schema once keeps revision `9003` truthful throughout the
milestone. The logbook view is proven by store tests in Chunk 1 and exposed
by the CLI in Chunk 3.

`store.Projects` joins `store.Tasks` in `internal/store`, sharing `store.DB`
and the package scan helpers. Single-entity mutations remain single atomic
statements whose predicates enforce the guards (project existence and
openness for containment; project openness for task transitions), with
classification queries producing `not_found` or `conflict` after a miss —
the established guard-and-classify pattern. Project resolution and recursive
deletion are the first multi-statement transactions. The project service
owns their scope and mutation sequence through `project.Store`'s
`WithinTransaction` callback; `store.Projects` implements that boundary with
`BEGIN IMMEDIATE`, a transaction-bound `store.Projects`, `COMMIT`, and
rollback on any failure. Resolution mutates the guarded project and then
cancels its open tasks; recursive deletion deletes the tasks and then the
project. Task mutations return affected rows ordered by `position, id`. The
per-entity split is code organization, not access control: `store.Projects`
writes the `tasks` table through its transaction-bound persistence methods.
List SQL is assembled only from validated enums and IDs; raw flag values
never become SQL.

### Command and output adapters

`projects` and `project` parent commands return usage errors when invoked
bare. Cobra owns flag declaration, changed-flag detection, mutual exclusion
(`--project`/`--no-project`), and construction of application intent; the
task and project services are wired through the existing injected factory,
which opens one `store.DB` and hands each service its store. Help, version,
argument parsing, and flag conflicts must not open the database. All success
and failure rendering flows through the shared writers.

## Test ownership

Each behavior has one primary owner at the cheapest layer that proves the
likely regression. Every added test must state the regression or explicit
requirement it protects and why its layer is the cheapest faithful proof.

- **Store tests with real temporary SQLite databases** own revision-`9003`
  bootstrap, `projects` constraints, container-scoped append positions
  (proven with interleaved inbox and project inserts asserting in-container
  position values, since whole-table MAX still looks ordered), view
  predicates (inbox containment exclusion, available project-open clause,
  logbook union and columns), re-parent append and same-container no-op,
  guard predicates and their `not_found`/`conflict` classification (using
  raw-SQL fixtures to create resolved projects before the CLI can), cascade
  and recursive-delete atomicity including rollback on failure, RESTRICT
  classification, and the shared cascade timestamp.
- **Service tests with store fakes** own semantic validation (IDs, titles,
  notes, status and exit enums, set-and-clear contradiction), edit-intent
  construction, error pass-through, and non-nil collection normalization.
  Fakes return canned values; they do not simulate transactions.
- **Command tests** own flag adaptation, mutual-exclusion and bare-noun
  usage errors with no database open, JSON envelope shapes (task
  `project_id`, `Resolution`, `Deletion`, logbook entries), human rendering
  (tables, the `Project` row, cascade narration including the omitted zero
  section, logbook ordering and local-day formatting), stream selection, and
  exit mapping. They do not restate store predicate matrices.
- **The Milestone 3 subprocess workflow in `e2e/`** owns confidence that the
  real binary persists containment across invocations and wires projects,
  cascade, deletion, and logbook together. Chunk 1 introduces it; Chunks 2
  and 3 extend it to the complete milestone workflow.

## Chunk 0 — Structural refactor (review-only)

### Why review-only

The `apperr` extraction, `Store` rename, and store split touch nearly every
file mechanically. Isolating them keeps Chunk 1's review about new behavior
instead of rename noise. `PROCESS.md` permits one such chunk; the next chunk
is human-facing. No demo is produced.

### Implementation

- [x] Extract `internal/apperr` (`Code`, code constants, `New`, and `CodeOf`)
      and migrate the task, store, and command layers to it.
- [x] Rename `task.Repository` to `task.Store` and update fakes and wiring.
- [x] Split `internal/store` into `store.DB` (open, pragmas, bootstrap,
      close, path resolution) and `store.Tasks` (task SQL only), construct
      task stores with `store.NewTasks`, and update the command factory.
- [x] Update `AGENTS.md` to the services/stores vocabulary.
- [x] Verify zero behavior change: no schema, SQL semantics, output, or exit
      differences; existing tests pass with only mechanical updates.
- [x] Run `make check` before opening the chunk pull request.

## Chunk 1 — Projects exist

### Human outcome

A human can create a project, put tasks in it, list a project's tasks, edit
and inspect projects, re-parent tasks between containers with append-on-move,
and see `inbox` exclude contained tasks.

### Implementation

- [x] Land the complete revision-`9003` schema: `projects` table,
      `tasks.project_id` with RESTRICT and index, updated `inbox` and
      `available` views, the `logbook` view, and the version bump. Prove
      bootstrap, constraints, and all view predicates with real SQLite,
      including the logbook union ahead of its command.
- [x] Prove container-scoped append with interleaved inserts asserting
      in-container position values.
- [x] Add `internal/project` (values, `Store` interface with `Add`, `Find`,
      `List`, `Edit`, service validation) and `store.Projects`.
- [x] Add `projects add`, `projects list` (status filter defaulting open),
      `project show`, and `project edit`; bare `gsd projects` and bare
      `gsd project` are usage errors, exit 2, without opening the database.
- [x] Carry `project_id` through the task value, JSON, scanner, and store
      round trips; add `--project` to task `add` and `list`; add mutually
      exclusive `--project`/`--no-project` to task `edit` with re-parent
      append and same-container no-op semantics.
- [x] Enforce membership guards at the store (nonexistent project →
      `not_found`; resolved project → `conflict`, proven with raw-SQL
      fixtures until Chunk 2 makes resolved projects reachable).
- [x] Add the `Project` row to human task `show`; keep collection rows
      unchanged.
- [x] Introduce the Milestone 3 subprocess workflow: containment, listing,
      inbox exclusion, and re-parent append across invocations.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/3-chunk-1.html`):

- [x] `gsd projects add "Kitchen reno"`, then `gsd add "Get quotes"
      --project 1`, `gsd add "Pick tiles" --project 1`, and a loose
      `gsd add "Buy milk"`.
- [x] `gsd list --project 1` shows the two project tasks; `gsd inbox` shows
      only the loose task.
- [x] `gsd projects list` and `gsd project show 1` render the project;
      `gsd project edit 1 --note "Budget: 20k"` persists.
- [x] `gsd show 2` displays the `Project` row; `gsd show 2 --json` contains
      `"project_id":1`.
- [x] `gsd projects add "Bathroom"`, `gsd edit 2 --project 2`, and
      `gsd list --project 2` shows it appended last; `gsd edit 2
      --no-project` returns it to the inbox end.
- [x] `gsd add "X" --project 99` fails `not_found` with exit 1; `gsd edit 2
      --project 1 --no-project` is a usage error with exit 2.
- [x] Bare `gsd projects` and bare `gsd project` are usage errors with
      exit 2.

## Chunk 2 — Lifecycle and cascade

### Human outcome

A human can complete or cancel a project and watch the cascade narrate every
open task it cancelled, reopen a project without undoing the cascade, and
delete projects with RESTRICT protection and an explicit recursive opt-in.

### Implementation

- [x] Extend `project.Store` with `Resolve`, `CancelOpenTasks`, `Reopen`,
      `Delete`, `DeleteTasks`, and `WithinTransaction`; have the project
      service build the `Resolution` and `Deletion` envelopes while owning
      the cascade and recursive-delete transaction scope and mutation
      sequence. Implement the transaction boundary and persistence primitives
      in `store.Projects` with guard-and-classify conflicts, one shared
      timestamp, `position, id` ordering, and rollback proof.
- [x] Add the resolved-project guard to task `done`, `cancel`, and `reopen`
      store predicates with `conflict` classification.
- [x] Add `project done`, `project cancel`, `project reopen`, and
      `project delete [--recursive]` commands.
- [x] Close the deferred Chunk 1 read-consistency finding when project
      deletion lands: make `list --project` derive project existence and task
      rows from one consistent SQLite snapshot while preserving the
      `not_found` versus empty-project distinction.
- [x] Close the deferred Chunk 1 error-guidance finding once resolved projects
      are user-reachable: report both resolved source and destination projects
      in one conflict, prove it with one focused store test, and show the
      failure and successful recovery in the chunk demo.
- [x] Render cascade and deletion narration (mutation line, task section
      omitted when empty) and the JSON envelopes.
- [x] Extend service, command, and store tests for the newly owned lifecycle
      behavior only.
- [x] Extend the subprocess workflow with cascade, guard, reopen, and
      deletion coverage.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/3-chunk-2.html`):

- [x] Build a project with three tasks; `gsd done` one; `gsd project done 1`
      prints the done line and a `Cancelled 2 open tasks:` section naming
      both.
- [x] `gsd project done 1` again fails `conflict` with exit 1; `gsd done`
      on a task inside the done project fails `conflict`.
- [x] `gsd add "Late idea" --project 1` fails `conflict`.
- [x] `gsd project reopen 1`: the cancelled tasks stay cancelled; `gsd
      reopen` one task, then `gsd project done 1` re-completes and cancels
      it again.
- [x] A project whose tasks are all resolved completes with only the done
      line — no cancelled section.
- [x] `gsd projects add "Doomed"` plus one task; `gsd project delete`
      fails `conflict`; `gsd project delete --recursive` succeeds and
      narrates the deleted task.
- [x] `gsd project cancel` on a fresh project shows the cancel narration;
      the subprocess workflow separately proves the compact
      `{"project":...,"cancelled_tasks":[...]}` JSON envelope so the demo
      remains human-readable as required by `plans/PROCESS.md`.
- [x] In a focused recovery example, create a task in one project, resolve both
      that source and a destination project, and show that re-parenting reports
      both blockers; reopen both projects and rerun the same edit successfully.

## Chunk 3 — Logbook

### Human outcome

A human can run `gsd logbook` and see finished tasks and projects
interleaved newest first, each row marked with its kind, status, and
resolution day.

### Implementation

- [ ] Add the `internal/logbook` read model (entry value, consumer-defined
      `Store` interface, thin service) and `store.Logbook` querying the view
      with `resolved_at` descending, project-before-task at the same
      instant, then `id` descending.
- [ ] Add the `logbook` command: JSON entries with the view column set;
      human rows of kind, id, title, status, and the local calendar day of
      `resolved_at`; empty logbook prints nothing.
- [ ] Extend the subprocess workflow to the complete milestone workflow,
      including logbook ordering and kinds.
- [ ] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/3-chunk-3.html`):

- [ ] Fresh database: `gsd logbook` prints nothing.
- [ ] Build the Kitchen reno story: project with two tasks, `gsd done` one,
      `gsd project done 1` cancelling the other.
- [ ] `gsd logbook` shows the project above its same-instant cancelled task,
      the independently completed task below, each row with kind, status,
      and day.
- [ ] `gsd logbook --json` entries carry `kind`, `resolved_at`, and
      `project_title` for task entries.

## Consolidation and exit

After every chunk is reviewed and squash-merged into the milestone branch:

- [ ] Reconcile the accepted semantics into the canonical specifications:
      the resolved-project guard, the recursive-delete envelope, logbook
      ordering and rendering, and `not_found` project references into
      `plans/COMMANDS.md` and `plans/OVERVIEW.md`.
- [ ] Verify `plans/OVERVIEW.md` § "Completion cascade and deletion" against
      shipped behavior word by word (the milestone's specific exit
      criterion).
- [ ] Confirm `plans/SCHEMA.md` matches the shipped views and resolve any
      temporary divergence recorded during implementation.
- [ ] Reconcile tests around stable observable contracts and codify lasting
      review findings in `AGENTS.md`, lint/build configuration, or the
      owning tests.
- [ ] Run the automated Milestone 3 subprocess workflow through
      `make check`.
- [ ] Build the real binary and retain a clean transcript of the workflow
      below.
- [ ] Have Javier demo the milestone user stories.
- [ ] Update the roadmap and links, then permanently delete
      `plans/MILESTONE_3.md` and root `PLAN.md` as required by
      `plans/PROCESS.md`.
- [ ] Rerun `make check`, require green final CI, review the consolidated
      milestone pull request, and squash-merge it into `main`.

## Agent-verified end-to-end workflow

Use the real built binary, a fresh temporary database under `.sandbox/`, and
JSON output except where human rendering is explicitly inspected.

- [ ] Create a project and three tasks in it plus one loose inbox task;
      assert each project task carries `"project_id"` and `gsd inbox`
      returns only the loose task while `gsd list --project N` returns all
      three in position order.
- [ ] `gsd done` one project task, then `gsd project done N`; assert the
      response contains the project row and exactly the two remaining open
      tasks as `cancelled_tasks`, all sharing one cascade timestamp.
- [ ] Assert `gsd done` on a task inside the done project, `gsd add
      --project` into it, and re-parenting into or out of it all fail
      `conflict` with exit 1, while editing a contained task's title
      succeeds.
- [ ] `gsd logbook` returns four entries (one project, three tasks), newest
      first with the project above its same-instant cancelled tasks, each
      with the correct `kind`.
- [ ] `gsd project reopen N`; assert the cancelled tasks stay cancelled
      (reopen is not un-cascade), then re-complete the project.
- [ ] `gsd projects add` plus one task; assert `gsd project delete` fails
      `conflict` (RESTRICT) and `gsd project delete --recursive` succeeds
      with the task in `deleted_tasks`.
- [ ] Re-parent a task between two projects; assert it appended last in the
      destination and that re-stating its container does not move it.
- [ ] Assert `gsd add --project 99` and `gsd list --project 99` fail
      `not_found` with exit 1.
- [ ] Assert bare `gsd projects` and bare `gsd project` are usage errors
      with exit code 2 and no database side effect.
- [ ] Inspect human output for one cascade (narrated section), one
      zero-cancel completion (no section), and the logbook (kind, status,
      local day, no ANSI styling).
- [ ] Run `make check` and report the command transcript and final clean
      result.
