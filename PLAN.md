# Milestone 2 Implementation Plan

This is the authoritative implementation plan for the outcomes in
[`plans/MILESTONE_2.md`](plans/MILESTONE_2.md). The milestone's capability,
command surface, date grammar, user stories, and acceptance workflow define
what must ship. Its implementation chunks are suggestions; the chunks below
define how the work will be delivered.

Data remains throwaway. Milestone 2 starts from the consolidated Milestone 1
capture loop and changes the development schema revision from `9001` to `9002`.
An older development database must fail with the existing conflict and deletion
guidance; there is no migration.

## Delivery shape

Milestone 2 is two sequential vertical slices. This is the chunk progress
checklist; check a chunk only after its implementation and verification items
below are complete:

- [x] **Chunk 1 — Deadlines** — a human can set, inspect, filter, change, and
      clear real due dates, including finding overdue open tasks.
- [x] **Chunk 2 — Deferral and availability** — a human can hide work until its
      local calendar day arrives and ask what is actionable now.

Both chunks cross the parser, application, SQLite, command, output, and test
boundaries needed for their outcome. Neither is an architectural-layer chunk,
and each ends with built-binary human proof. The second chunk starts only after
the first is reviewed and squash-merged into the milestone branch.

Expected review size, including tests:

- Chunk 1: approximately 750–1,100 changed lines across the shared date
  foundation and deadline behavior.
- Chunk 2: approximately 600–900 changed lines, reusing that foundation for
  deferral and availability.

These are review-size guides, not targets. If a chunk grows enough to obscure
its outcome, revise this plan before splitting it. A split may introduce one
review-only chunk, but it may not create two consecutive chunks without a new
human-verifiable capability.

## Settled behavior

### Calendar input

The pure date parser accepts only:

- canonical `YYYY-MM-DD`;
- `today` and `tomorrow`;
- `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, and `sun`, each resolving to the
  next occurrence rather than the current day; and
- `+Nd` and `+Nw` relative forms.

Resolution uses the local calendar represented by the injected reference time.
Every accepted value becomes canonical `YYYY-MM-DD` before reaching the
repository. Impossible dates, noncanonical dates, unsupported phrases, and
malformed relative forms return `invalid_argument`; the parser never guesses.

### Fields and clearing

`due_on` and `defer_until` are nullable calendar-day fields in application and
JSON task values. Add accepts optional `--due` and `--defer`. Edit distinguishes
three states for each field: omitted, set to a parsed date, and explicitly
cleared with `--no-due` or `--no-defer`.

A set flag and its matching clear flag are mutually exclusive command grammar.
That conflict is a usage error with exit code 2 and is detected before opening
the database. The service also rejects an internally contradictory field
change so non-CLI callers cannot bypass the use-case invariant.

### List filters

`list` keeps its existing default status of `open`. Its date selectors mean:

- `--due`: selected-status tasks whose `due_on` is set;
- `--overdue`: selected-status tasks that are also open and have
  `due_on < date('now','localtime')`; and
- `--deferred`: selected-status tasks with
  `defer_until > date('now','localtime')`.

The three date selectors are mutually exclusive and each composes with
`--status`. A contradictory combination such as `--status done --overdue`
succeeds with an empty collection. Date filters do not change position-then-ID
ordering.

### Availability

The reduced Milestone 2 `available` view and command return open tasks whose
`defer_until` is null or no later than `date('now','localtime')`. Relationship
clauses and enrichment remain deferred to their owning milestones. The command
returns the same task shape and ordering contract as other task collections.

### Output

JSON remains one compact, newline-terminated value. Task objects expose
`due_on` and `defer_until` as canonical strings or `null`; collections remain
arrays, including `[]`.

Human `show` output gains labeled due and defer rows. Human task collections
append `due DATE` and `defer DATE` tokens only when those values are present;
when both exist, due precedes defer. Existing concise mutation output remains
unchanged. Milestone 2 adds no color or ANSI styling: human output stays
unstyled until the Config milestone establishes color behavior.

## Implementation boundaries

### Date engine

Add a dependency-free `internal/dates` package with a pure parser that takes an
explicit reference time. Keep grammar recognition, calendar arithmetic, and
canonical formatting there. It returns ordinary errors that the task service
maps to the stable `invalid_argument` application code with the rejected input
in the diagnostic.

The task service samples its clock once for an add or edit request that contains
date input and passes that reference to every date parse in the request. This
keeps two fields in one mutation on the same local calendar day.

### Task application

Extend the task value and add/edit fields for nullable due and defer dates.
Represent edit intent explicitly so omission and clearing cannot collapse into
the same value. Introduce a list-options value containing status and one
allowlisted date selector, and add `Available` to the application and repository
interfaces.

The service owns parsing, field-change validation, date-selector validation,
and normalization of nil collections. It delegates only canonical values and
validated options to the repository.

### SQLite store

Establish the complete Milestone 2 schema in Chunk 1:

- add nullable `defer_until` and `due_on` columns and their round-trip checks;
- create the reduced `available` view; and
- set `PRAGMA user_version = 9002`.

Landing the final schema once makes revision `9002` truthful throughout the
milestone and avoids an intermediate or same-number schema revision. Chunk 1
therefore proves the installed available-view predicate with real SQLite and
freezes its SQL definition; Chunk 2 exposes rather than revises it. Deferral
remains unavailable through the CLI until Chunk 2 even though its storage and
view foundation exists.

Update the shared task projection and scanner together so every store path
round-trips both nullable fields. Add and edit continue to use atomic SQL
statements. List SQL is assembled only from the validated status and date
selector enums; raw flag values never become SQL. `Available` selects from the
view and orders by `position, id`.

### Command and output adapters

Cobra owns flag declaration, changed-flag detection, mutual-exclusion grammar,
and construction of application field intent. Add and edit gain their date
flags in their owning chunks. List gains the deadline selectors in Chunk 1 and
the deferred selector in Chunk 2. Register `available` only in Chunk 2.

All success and failure rendering continues through the shared writers. Help,
version, argument parsing, and flag conflicts must not open the database.

## Test ownership

Each behavior has one primary owner at the cheapest layer that proves the
likely regression:

- **Date parser unit tests** own the accepted grammar, canonicalization,
  next-weekday semantics, local-calendar arithmetic, impossible dates,
  noncanonical input, malformed relative forms, and boundary arithmetic. A
  pure table-driven test controls reference time and location and proves this
  more faithfully than command fixtures.
- **Task service tests** own one-clock-sample parsing orchestration, coded
  semantic errors, set/omit/clear intent, invalid selector rejection, and
  non-nil collection normalization. Repository fakes observe only the
  canonical values crossing the real boundary.
- **Store tests with real temporary SQLite databases** own schema revision and
  checks, nullable round trips across every mutation, atomic edit behavior,
  list predicates, available-view semantics, and ordering. They use dates far
  enough in the past or future where the exact boundary is irrelevant. For the
  equality boundary, the test captures SQLite's local date before arranging
  fixtures, rechecks it after the assertion, and rebuilds the case from the new
  date if midnight rolled over. Store tests do not repeat the parser grammar
  matrix.
- **Command tests** own flag adaptation, mutual-exclusion usage errors,
  no-database-open behavior for syntax failures, JSON shapes, human date
  rendering, stream selection, and exit mapping. They do not restate store
  predicate matrices.
- **A dedicated Milestone 2 subprocess workflow** owns confidence that the real
  binary persists canonical dates across invocations and wires add, edit,
  list, and available together. Chunk 1 introduces the deadline portion;
  Chunk 2 extends it to the completed milestone workflow.

Every added test must state the regression or explicit requirement it protects
and why its layer is the cheapest faithful proof before implementation begins.

## Chunk 1 — Deadlines

### Human outcome

A human can assign a due date using the closed grammar, inspect its canonical
value, list all due work, find overdue open work, change the deadline, and clear
it.

### Implementation

- [x] Add the pure date parser and its controlled-clock grammar tests.
- [x] Install the complete revision-`9002` schema foundation: both date
      columns, checks, reduced available view, projections, and scanners. With
      a real temporary SQLite database, prove bootstrap plus the final view
      predicate for null, past, today, and future defer dates and for non-open
      tasks. Derive the equality fixture from SQLite's captured local date,
      recheck that date after the assertion, and rebuild the case if midnight
      rolled over.
- [x] Carry nullable date values through task JSON and store round trips.
- [x] Add due-date intent to add/edit, parse it in the service, and persist set
      and clear operations atomically.
- [x] Add `--due` to add; add mutually exclusive `--due`/`--no-due` to edit.
- [x] Add mutually exclusive `list --due` and `list --overdue`, including
      service options and allowlisted store predicates.
- [x] Add labeled/compact human date rendering without styling.
- [x] Introduce the dedicated subprocess workflow for canonical deadline
      persistence, filtering, clearing, and rejection behavior.
- [x] Run `make check` and build the real binary before opening the chunk pull
      request.

### Human proof

Against a fresh database with the real built binary:

- [x] Add a task with `--due tomorrow`; verify JSON and `show` contain the same
      canonical local date.
- [x] Verify `list --due` contains it and `list --overdue` does not.
- [x] Independently compute a canonical local yesterday, edit the task to that
      date, and verify `list --overdue` contains it.
- [x] Complete the task; verify the default open overdue list excludes it and
      `list --status done --overdue` succeeds empty.
- [x] Reopen it, clear the deadline with `--no-due`, and verify due filtering no
      longer returns it.
- [x] Reject `2026-02-30`, `2026-8-3`, `next tuesday`, and `+3x` as structured
      `invalid_argument` errors with exit code 1.
- [x] Supply conflicting list date selectors; verify a usage error with exit
      code 2 and no database side effect.

## Chunk 2 — Deferral and availability

### Human outcome

A human can defer a task until a local calendar day, list deliberately deferred
work, see only actionable tasks in `available`, and clear a defer date to make a
task immediately available.

### Implementation

- [x] Add defer-date set/omit/clear orchestration using the shared parser and
      revision-`9002` storage established in Chunk 1.
- [x] Add `--defer` to add; add mutually exclusive `--defer`/`--no-defer` to
      edit.
- [x] Add `list --deferred` and make it mutually exclusive with the two
      deadline selectors while retaining status composition.
- [x] Expose the reduced available view through repository, service, Cobra,
      JSON, and human output.
- [x] Extend command, service, store, and rendering tests only for the newly
      owned deferral and availability behavior.
- [x] Extend the dedicated subprocess workflow to the complete Milestone 2
      workflow.
- [x] Run `make check` and build the real binary before opening the chunk pull
      request.

### Human proof

Against a fresh database with the real built binary:

- [x] Add undated, `--defer today`, `--defer tomorrow`, and `--defer +1w`
      tasks.
- [x] Verify JSON stores canonical values for all dated tasks.
- [x] Verify `available` contains the undated and today-deferred tasks but
      excludes tomorrow and `+1w`.
- [x] Verify `list --deferred` contains the two future-deferred tasks.
- [x] Clear tomorrow's defer date with `edit --no-defer`; verify it immediately
      appears in `available` and leaves the deferred list.
- [x] Complete a future-deferred task; verify default `list --deferred` excludes
      it while `list --status done --deferred` includes it.
- [x] Verify due and defer can coexist on one task, both appear in JSON and
      human output, and selecting multiple date filters is a usage error.

## Consolidation and exit

After both chunks are reviewed and squash-merged into the milestone branch:

- [ ] Reconcile the accepted date-filter meanings and mutual exclusion into
      `plans/COMMANDS.md`; retain its existing no-color-before-Milestone-6 rule.
- [ ] Confirm `plans/OVERVIEW.md` and `plans/SCHEMA.md` match shipped behavior
      and resolve any temporary divergence discovered during implementation.
- [ ] Reconcile tests around stable observable contracts and codify lasting
      review findings in `AGENTS.md`, lint/build configuration, or the owning
      tests.
- [ ] Run the automated Milestone 2 subprocess workflow through `make check`.
- [ ] Build the real binary and retain a clean transcript of the workflow
      below.
- [ ] Have Javier demo the milestone user stories.
- [ ] Update the roadmap and links, then permanently delete
      `plans/MILESTONE_2.md` and root `PLAN.md` as required by
      `plans/PROCESS.md`.
- [ ] Rerun `make check`, require green final CI, review the consolidated
      milestone pull request, and squash-merge it into `main`.

## Agent-verified end-to-end workflow

Use the real built binary, a fresh temporary database under `.sandbox/`, and
JSON output except where human rendering is explicitly inspected. Capture the
local calendar date before the first date-sensitive command, recheck it after
the final date-sensitive assertion, and restart with a fresh database if the
date rolled over while the workflow ran.

- [x] Use the captured local date for independent expectation calculations.
      Add tasks with `--defer today`, `--defer tomorrow`, `--defer +1w`, and
      `--due today`; assert every stored non-null date is the independently
      computed canonical value.
- [x] Run `available`; assert it contains the today-deferred and due-today tasks
      and excludes the tomorrow and `+1w` deferred tasks.
- [x] Run `edit --no-defer` on the tomorrow-deferred task; assert it now appears
      in `available` and no longer appears in `list --deferred`.
- [x] Assert `list --overdue` is empty. Independently compute canonical
      yesterday, add an open task due that day, and assert only that task
      appears overdue.
- [x] Complete the overdue task; assert default overdue output is empty and
      `list --status done --overdue` succeeds with an empty JSON array.
- [x] Assert `list --due`, `list --overdue`, and `list --deferred` reject every
      pairwise combination as usage errors with exit code 2.
- [x] Reject `2026-02-30`, `2026-8-3`, `next tuesday`, and `+3x` on date-bearing
      mutations; assert structured `invalid_argument` errors on stderr and exit
      code 1.
- [x] For each weekday token, compute the next occurrence independently from
      the captured local date, add a dated task, and assert the stored canonical
      date.
- [x] Inspect human `show`, `list`, and `available`; assert labeled or compact
      date output as appropriate and no ANSI styling.
- [x] Run `make check` and report the command transcript and final clean result.
