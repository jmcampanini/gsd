# Milestone 5 Implementation Plan

This is the authoritative implementation plan for the outcomes in
[`plans/MILESTONE_5.md`](plans/MILESTONE_5.md). The milestone's capability,
command surface, semantics, user stories, and acceptance workflow define what
must ship. Its implementation chunks are suggestions; the chunks below define
how the work will be delivered. This plan is temporary and is retired at
consolidation.

Data remains throwaway. Milestone 5 starts from the consolidated Milestone 4
areas baseline and changes the development schema revision from `9004` to
`9005` — the last bump; this revision becomes the schema baseline used at Go
live. An older or nonempty development database must fail with the existing
conflict and deletion guidance; there is no migration.

## Delivery shape

Milestone 5 is three sequential chunks. This is the chunk progress checklist;
check a chunk only after its implementation and verification items below are
complete:

- [x] **Chunk 0 — Transaction seam** — review-only: guarded task and project
      mutations move to read-then-validate transactions behind the `Tasks`
      executor seam, the duplicated classification stanzas and the
      sentinel-row listings are retired, and the triplicated service helpers
      extract to `internal/domain`. No behavior changes.
- [x] **Chunk 1 — Tag administration** — a human can create, list, rename,
      and delete tags in one case-insensitive namespace, with the complete
      revision-`9005` schema landed and the view `tags` enrichment proven by
      store tests.
- [x] **Chunk 2 — Attachment everywhere** — a human can tag and untag tasks,
      projects, and areas, create any of them pre-tagged with `--tag`, filter
      the task list by tag, and see the `tags` array complete the JSON output
      contract.

Chunk 0 is the milestone's one review-only chunk; the milestone file mandates
its refactor before any tag behavior lands, and it delivers no new
human-facing capability. Chunks 1 and 2 cross the application, SQLite,
command, output, and test boundaries needed for their outcomes and end with
built-binary human proof captured as chunk demos
(`.sandbox/demos/5-chunk-1.html`, `.sandbox/demos/5-chunk-2.html`). Each
chunk starts only after the previous one is reviewed and squash-merged into
the milestone branch.

Expected review size, including tests:

- Chunk 0: approximately 1,600–2,200 changed lines across the task and
  project stores, the transaction seam, the shared helper extraction, and
  the retired sentinel listings.
- Chunk 1: approximately 1,700–2,000 changed lines across the schema, the new
  tag application, commands, and view tests.
- Chunk 2: approximately 1,500–2,000 changed lines across the three entity
  applications and stores, attachment commands and flags, JSON and view
  enrichment, and the end-to-end workflow.

These are review-size guides, not targets. If a chunk grows enough to
obscure its outcome, revise this plan before splitting it.

## Settled behavior

### Guard architecture (the chunk 0 decision)

Guarded mutations use **read-then-validate inside the transaction**: the
store reads the entity and its container state under `BEGIN IMMEDIATE`,
validates in Go with the existing typed errors, then executes plain writes.
The former classification pass (`findContainerState`, the blockers helpers)
is promoted from after-the-fact forensics to the single guard authority;
guard predicates leave the mutation SQL. `BEGIN IMMEDIATE` holds the write
lock across read, validation, and write, so the read is authoritative.

This applies to every mutation that carries container guards and a
classification pass: task `add`, `edit`, `done`, `cancel`, `reopen` and
project `add`, `edit`, `resolve` (done/cancel), `reopen`. Project `delete`
keeps its guarded statement, but runs missing-versus-contained classification
inside the same `BEGIN IMMEDIATE` transaction so that result is authoritative.
Operations whose mutation is proven complete by one guarded atomic statement
with a single self-classifying predicate — task delete, tag rename, area
archive/unarchive/delete — keep that mutation shape per `AGENTS.md`. When
failure classification needs follow-up reads, the store holds the statement
and those reads in one immediate transaction so the classification is
authoritative.

### Tag identity and namespace

Tags are addressed by name, never ID; the name is the identity everywhere a
tag appears on the command line. Names follow title validation: valid UTF-8,
nonblank after surrounding-space inspection, stored unchanged. Uniqueness is
case-insensitive through SQLite `NOCASE`, which folds ASCII only: creating
`Errands` when `errands` exists is `conflict`, while non-ASCII case variants
remain distinct. The conflict message names the stored spelling. The first-created spelling is what displays; attach, detach,
filter, rename-source, and delete-target matching are all case-insensitive.
Renaming a tag to a different casing of itself succeeds; renaming onto a
different existing tag is `conflict`. Tags must pre-exist: `tag`, `untag`,
`--tag` on the three `add` commands, and `list --tag` with an unknown name
are `not_found` (`no tag NAME`); `gsd tags add` is the only creation path.

### Attachment semantics

- Attach and detach are both idempotent no-op successes: tagging with a tag
  already attached and untagging a tag not currently attached succeed
  without changing join rows. Unknown names remain `not_found`.
- Multi-name commands (`gsd tag N A B C`, `untag`, repeated `--tag`) are
  all-or-nothing: one unknown name fails the whole command and nothing is
  attached, detached, or created. Duplicate names within one command
  (including case variants) collapse to one attachment.
- `--tag` on `add` makes creation transactional: insert the entity, resolve
  every tag name, insert the join rows; any unknown name is `not_found` and
  no entity is created.
- Tagging is a content edit, never a guarded mutation: attach and detach
  succeed regardless of archived governing areas or resolved projects,
  matching the content-edits-stay-allowed rule and `tags delete`'s
  unconditional detachment.
- Attach and detach never touch the entity row — no `updated_at` bump; join
  rows are bookkeeping, not entities. `tags rename` bumps the tag's own
  `updated_at`; attach and detach do not.

### Deletion and rename

`tags delete NAME` deletes the tag; join-row CASCADE detaches it from every
task, project, and area. The command reports the total detached count.
`tags rename OLD NEW` renames in place; every entity's `tags` array reflects
the new name immediately because entities store join rows, not copies of the
name. Deleting a tagged entity removes its join rows through the entity-side
CASCADE, visible as dropped usage counts.

### Output

JSON remains one compact, newline-terminated value. Every task, project, and
area JSON row — mutations, `show`, lists, and view output — gains `tags`, an
array of tag names and the output contract's final field. Entity `tags`
arrays are produced by the same correlated `json_group_array` subquery the
`SCHEMA.md` views use, so direct output and view output agree; aggregate input
is explicitly ordered by tag ID, making array order tag-creation order. Tag rows are their table row (`id`, `title`, `created_at`, `updated_at`);
`tags list` rows additionally carry `usage_count`, the total attachment count
across all three entity kinds.
Envelopes: `tag`/`untag` and the three `add` commands echo the affected
entity row (with `tags`); `tags add` and `tags rename` echo the tag row;
`tags delete` returns `{"tag":{...},"detached":N}`.

Human output: `tags list` is a headerless aligned table of name and usage
count, ordered alphabetically case-insensitively — no IDs, because tags are
never addressed by ID. Task, project, and area `show` gain a `Tags` row with
comma-separated names, blank when untagged; human collection rows gain no
tags column. Tag mutation lines use stored spelling after case-insensitive
resolution; rename renders both the stored previous title and the stored new
title rather than echoing command arguments. Mutation lines stay concise and
action-prefixed: `Added tag errands`, `Renamed tag errands to out-and-about`,
`Deleted tag out-and-about (detached from 3 items)`,
`Tagged: task 7  errands`, `Untagged: task 7  errands`. Bare `gsd tags` is a
usage error with exit code 2 and must not open the database; bare `gsd tag`
and `gsd untag` without arguments are usage errors likewise.

## Implementation boundaries

### Chunk 0 refactor

- `store.Tasks` gains the executor + `WithinTransaction` shape its siblings
  have; every read the guarded mutations perform, including the former
  classification reads, routes through the executor so in-transaction
  validation sees transaction state.
- The guarded task and project mutations convert to read-then-validate:
  find the row, read container state, validate in Go returning the existing
  typed errors (`not_found`, blockers `conflict` via `blockers.go`), then
  plain writes. `Tasks.Edit` decomposes: read task, compute destination,
  validate source and destination states, one small UPDATE. The `Projects`
  store's duplicated find-row → find-area → archived-conflict stanzas in
  `Resolve` and `Reopen`, and `Edit`'s re-implementation of the blocker
  message, dissolve into the shared in-transaction validation.
- The sentinel-row idiom in `Tasks.listContained` and the area-scoped
  project listing retires: each contained listing becomes a container `Find`
  followed by the list query inside one deferred read transaction — same
  missing-versus-empty semantics with snapshot consistency and no fabricated
  all-zero row.
- The triplicated service helpers — title/note validation, positive-ID
  checks, digit-strict ID parsing, timestamp formatting, nil-to-empty slice
  normalization — extract to a new `internal/domain` package, parameterized
  by entity noun so every message stays byte-identical. The `tag` package
  never mints copy four.
- No behavior changes: the existing store, service, command, and e2e suites
  pass with at most mechanical updates (helper import paths); observable
  contracts are untouched.

### Tag application

New `internal/tag` package following the entity package shape: a `Tag` value
(table-row JSON), a `ListedTag` value embedding `Tag` plus
`UsageCount int64` (`json:"usage_count"`), and a consumer-defined
`tag.Store` interface: `Add`, `Find` (by name, case-insensitive), `List`
(with usage counts), `Rename`, `CountUsage`, `Delete`, plus
`WithinTransaction(context.Context, func(Store) error) error`. The service
owns name validation through `internal/domain`, timestamping, the rename
transaction (find stored source → rename), and the delete-with-count
transaction (find → count usage → delete). `Add` and `Rename` each keep one
guarded mutation statement; immediate transactions make their simple
find-based conflict classification authoritative. The rename result carries
the stored previous title for human output while JSON remains the updated tag
row.

### Entity applications

`task.Task`, `project.Project`, and `area.Area` gain
`Tags []string` (`json:"tags"`, never null). Each service gains
`Tag(ctx, id, names)` and `Untag(ctx, id, names)` owning the all-or-nothing
transaction: find the entity, resolve names, attach or detach, return the
refreshed entity. Each `AddFields` gains `Tags []string`; the creation
transaction inserts the entity, resolves names, and inserts join rows.
`task.ListOptions` gains an optional `Tag *string` filter composing with the
existing status, date, and container filters. Services normalize multi-name
input (trim-validate each name, collapse case-insensitive duplicates) before
the store sees it. Each entity's store interface gains narrow tag methods
(`ResolveTags`, `AttachTags`, `DetachTags`, and tag-aware reads); tag
knowledge stays behind the consumer-defined interfaces.

### SQLite store

Chunk 1 establishes the complete Milestone 5 schema in one revision:

- `tags` table (`UNIQUE COLLATE NOCASE`) and the three join tables
  (`task_tags`, `project_tags`, `area_tags`) with composite primary keys,
  `WITHOUT ROWID`, CASCADE on both sides, and the three reverse-lookup
  indexes — exactly per `SCHEMA.md`;
- the three views rebuilt with the `tags` JSON-array enrichment column,
  byte-matching `SCHEMA.md`; and
- `PRAGMA user_version = 9005`.

Landing the final schema once keeps revision `9005` truthful throughout the
milestone; the view `tags` column and usage counts are proven in Chunk 1 by
store tests with raw-SQL join-row fixtures ahead of their CLI surface, the
established pattern. `store.Tags` joins the per-entity stores sharing
`store.DB` and the seam. Attachment uses `INSERT OR IGNORE` for idempotent
join rows and plain `DELETE` for detach; name resolution is one shared
case-insensitive lookup written once in the store package. Entity queries
gain the correlated `json_group_array` subquery with aggregate input explicitly
ordered by tag ID and scan the JSON text into `[]string` (empty array when
untagged). `list --tag` resolves the tag
in-transaction (`not_found` on a miss) and filters through the join table.

### Command and output adapters

`cmd` gains the `tags` parent command (`add`, `list`, `rename`, `delete`),
task-noun `tag`/`untag` verbs, `project tag`/`project untag`,
`area tag`/`area untag`, repeatable `--tag` on `gsd add`, `projects add`,
and `areas add` (string-array flag), and `--tag` on `gsd list`. Cobra owns
argument arity and flag declaration; the services own semantic validation;
runtime dependencies open only through the injected factory, and help,
version, and usage failures must not open the database. All success and
failure rendering flows through the shared writers; recovery guidance naming
flags or command spellings composes at the command adapter.

## Test ownership

Each behavior has one primary owner at the cheapest layer that proves the
likely regression. Every added test must state the regression or explicit
requirement it protects and why its layer is the cheapest faithful proof.

- **Store tests with real temporary SQLite databases** own revision-`9005`
  bootstrap, the tags DDL (NOCASE uniqueness including case-only rename,
  composite-PK idempotency, CASCADE in both directions, reverse-lookup
  indexes), view `tags` enrichment via raw-SQL fixtures, usage-count
  aggregation across the three kinds, alphabetical NOCASE list ordering,
  case-insensitive resolution and filtering, read-then-validate semantics
  (typed `not_found`/`conflict` classification from in-transaction reads,
  multi-blocker gathering, and rollback proof: a failed resolve mid-attach
  or mid-create leaves no entity and no join rows), the retired sentinel
  listings' missing-versus-empty semantics, and the chunk 0 no-behavior-
  change contract via the existing untouched suites.
- **Service tests with store fakes** own name/ID/title validation through
  `internal/domain`, multi-name normalization and case-insensitive
  duplicate collapse, all-or-nothing transaction scope ownership, the
  delete-with-count envelope, error pass-through, and non-nil `tags` and
  collection normalization. Fakes return canned values; they do not
  simulate transactions.
- **Command tests** own flag adaptation (`--tag` accumulation, arity and
  usage errors with no database open), JSON envelope shapes (`tags` arrays
  on entity rows, `usage_count` rows, the `detached` envelope), human
  rendering (the tags table, `Tags` rows, mutation lines), stream
  selection, and exit mapping. They do not restate store matrices.
- **The Milestone 5 subprocess workflow in `e2e/`** owns confidence that
  the real binary persists the tag namespace and attachments across
  invocations and wires administration, attachment, filtering, rename
  propagation, and CASCADE detachment together. Chunk 1 introduces it;
  Chunk 2 extends it to the complete milestone workflow.

## Chunk 0 — Transaction seam

### Review-only rationale

Chunk 0 delivers no new human-facing capability; the milestone file mandates
this refactor before any tag behavior lands, and `PROCESS.md` permits a
review-only chunk when it is not consecutive with another. No chunk demo.

### Implementation

- [x] Give `store.Tasks` the executor + `WithinTransaction` shape and route
      every mutation-supporting read, including container-state reads,
      through the executor.
- [x] Convert task `add`, `edit`, `done`, `cancel`, and `reopen` to
      read-then-validate transactions returning the existing typed errors;
      decompose `Tasks.Edit` (read task, compute destination, validate
      states, plain UPDATE) and delete the statement-guard predicates and
      the forensic classification pass they duplicated.
- [x] Convert project `add`, `edit`, `resolve`, and `reopen` likewise,
      dissolving the duplicated classification stanzas and `Edit`'s
      re-implemented blocker message into shared in-transaction validation;
      run project `delete` classification in the guarded statement's
      transaction as well.
- [x] Retire the sentinel-row idiom: contained task and project listings
      become container `Find` plus list query inside one deferred transaction
      with identical missing-versus-empty semantics.
- [x] Extract the triplicated service helpers into `internal/domain`,
      parameterized by entity noun with byte-identical messages, and point
      the task, project, and area services at it.
- [x] Prove no behavior change: existing store, service, command, and e2e
      suites pass with only mechanical import updates; add rollback-proof
      store tests for the converted transactions.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

## Chunk 1 — Tag administration

### Human outcome

A human can create tags into one case-insensitive namespace, list them
alphabetically with usage counts, rename them, and delete them, with the
complete revision-`9005` schema live and byte-matching `SCHEMA.md`.

### Implementation

- [x] Land the complete revision-`9005` schema: `tags`, the three join
      tables with composite PKs / `WITHOUT ROWID` / CASCADE, the three
      reverse-lookup indexes, the rebuilt views with the `tags` column, and
      the version bump. Prove bootstrap, constraints, CASCADE both
      directions, view enrichment, and usage counts with real SQLite and
      raw-SQL join-row fixtures ahead of the CLI surface.
- [x] Add `internal/tag` (values, `Store` interface, service validation
      through `internal/domain`, the delete-with-count transaction) and
      `store.Tags` with case-insensitive resolution written once.
- [x] Add `gsd tags add NAME`, `gsd tags list`, `gsd tags rename OLD NEW`,
      and `gsd tags delete NAME`; bare `gsd tags` is a usage error, exit 2,
      without opening the database.
- [x] Render tag output: JSON tag rows, `usage_count` list rows, the
      `detached` envelope; human name/count table and the mutation lines,
      with conflicts naming the stored spelling.
- [x] Introduce the Milestone 5 subprocess workflow: tag administration
      persistence across invocations.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/5-chunk-1.html`):

- [x] `gsd tags add errands`, `gsd tags add home`; `gsd tags list` shows
      both alphabetically with count 0.
- [x] `gsd tags add Errands` fails `conflict` with exit 1, naming the
      stored spelling `errands`.
- [x] `gsd tags rename errands out-and-about` prints the mutation line;
      `gsd tags list` reflects it. `gsd tags rename home HOME` succeeds
      (case-only rename).
- [x] `gsd tags delete out-and-about` prints
      `Deleted tag out-and-about (detached from 0 items)`.
- [x] `gsd tags rename ghost x` and `gsd tags delete ghost` fail
      `not_found` with exit 1; `gsd tags add ""` fails `invalid_argument`
      with exit 1.
- [x] Bare `gsd tags` is a usage error with exit code 2.

## Chunk 2 — Attachment everywhere

### Human outcome

A human can tag and untag tasks, projects, and areas by name, create all
three pre-tagged with `--tag`, filter the task list by tag, watch renames
propagate and deletes detach everywhere, and see the `tags` array complete
every JSON row — finishing the v1 entity model and output contract.

### Implementation

- [x] Carry `Tags []string` through the task, project, and area values,
      scanners, and every store round trip using the views' correlated
      subquery; view-row scans gain the `tags` column.
- [x] Add `Tag`/`Untag` to the three services with the all-or-nothing
      transaction, idempotent attach and detach, case-insensitive
      resolution, duplicate collapse, and no `updated_at` bump; extend the
      store interfaces with the narrow tag methods.
- [x] Add `Tags` to the three `AddFields` and make creation transactional:
      unknown tag → `not_found`, no entity created, proven by rollback
      tests.
- [x] Add the `--tag` filter to `task.ListOptions` and `gsd list`,
      resolving the tag in-transaction and composing with existing filters.
- [x] Add the commands: `gsd tag N NAME...`, `gsd untag N NAME...`,
      `gsd project tag/untag N NAME...`, `gsd area tag/untag N NAME...`,
      repeatable `--tag` on the three `add` commands.
- [x] Render attachment output: entity echoes with `tags`, the `Tags` row
      on the three `show` commands, and the `Tagged:`/`Untagged:` mutation
      lines; tagging succeeds under archived areas and resolved projects.
- [x] Extend the subprocess workflow to the complete milestone workflow:
      attachment persistence, filtering, rename propagation, tag deletion
      detachment, and entity-deletion CASCADE visible in usage counts.
- [x] Run `make check` and build the real binary before opening the chunk
      pull request.

### Human proof

Against a fresh database with the real built binary, captured as the chunk
demo (`.sandbox/demos/5-chunk-2.html`):

- [x] `gsd tags add errands`; `gsd add "Drop off dry cleaning"`;
      `gsd projects add "Kitchen reno"`; `gsd areas add "Home"`.
- [x] `gsd tag 1 errands`, `gsd project tag 1 errands`,
      `gsd area tag 1 errands`; `gsd tags list` shows `errands 3`.
- [x] `gsd list --tag errands` shows the task; `gsd show 1` renders the
      `Tags` row.
- [x] `gsd tag 1 groceries` fails `not_found` (`no tag groceries`) with
      exit 1; `gsd tag 1 ERRANDS` succeeds as a no-op; `gsd untag 1 ghost`
      fails `not_found`.
- [x] `gsd add "Buy stamps" --tag errands` echoes the task with its tag;
      `gsd add "X" --tag nope` fails `not_found` and creates nothing
      (`gsd inbox` unchanged).
- [x] `gsd tags rename errands out-and-about`; `gsd show 1` shows the new
      name.
- [x] `gsd tags delete out-and-about` reports detachment from 4 items;
      `gsd show 1`, `gsd project show 1`, and `gsd area show 1` all show no
      tags, entities intact.

## Consolidation and exit

After every chunk is reviewed and squash-merged into the milestone branch:

- [ ] Reconcile the accepted semantics into the canonical specifications:
      idempotent attach and detach, all-or-nothing multi-name commands,
      unguarded tagging as a content edit, no `updated_at` bump on
      attachment, the `usage_count` and `detached` output shapes, the
      `tags` array as the contract's final field, and the human rendering
      into `plans/COMMANDS.md` and `plans/OVERVIEW.md`.
- [ ] **Schema convergence audit**: dump the dev database schema and diff
      against `SCHEMA.md`'s DDL — tables, checks, indexes, and views must
      be byte-comparable. Fix any difference or record it in
      `plans/DIVERGENCES.md` and reconcile it before Config begins.
- [ ] Reconcile tests around stable observable contracts and codify lasting
      review findings in `AGENTS.md`, lint/build configuration, or the
      owning tests.
- [ ] Run the automated Milestone 5 subprocess workflow through
      `make check`.
- [ ] Drive the real built binary through the agent-verified workflow below
      and retain a clean transcript.
- [ ] Have Javier run `/review-foundation` in a Fable session against the
      milestone branch and resolve the accepted findings.
- [ ] Have Javier demo the milestone user stories.
- [ ] Update the roadmap and links, then permanently delete
      `plans/MILESTONE_5.md` and root `PLAN.md` as required by
      `plans/PROCESS.md`.
- [ ] Rerun `make check`, require green final CI, review the consolidated
      milestone pull request, and squash-merge it into `main`.

## Agent-verified end-to-end workflow

Use the real built binary, a fresh temporary database under `.sandbox/`, and
JSON output except where human rendering is explicitly inspected.

- [ ] Create tags `errands` and `home`; create a task, a project, and an
      area; attach `errands` to all three. Assert each echo carries the
      `tags` array and `gsd tags list` shows `errands` with `usage_count`
      3 and `home` with 0.
- [ ] Unknown-name matrix: `gsd tag`, `gsd untag`, `gsd project tag`,
      `gsd area tag`, `gsd add --tag`, and `gsd list --tag` with an unknown
      name all fail `not_found` with exit 1; assert the multi-name command
      `gsd tag N errands ghost` attaches nothing.
- [ ] Case-insensitivity: `gsd tags add Errands` fails `conflict` naming
      `errands`; `gsd tag N ERRANDS` attaches the existing tag; assert the
      entity's `tags` array shows the stored spelling.
- [ ] Duplicate attach: re-tag an already-tagged entity; assert success and
      `usage_count` unchanged. Untag a not-attached existing tag; assert
      success and counts unchanged.
- [ ] `gsd tags rename errands out-and-about`: assert the `tags` arrays on
      the task, project, and area all show the new name.
- [ ] `gsd tags delete out-and-about`: assert the `detached` count, then
      `show` on all three entities has the tag gone with entities intact.
- [ ] Delete a tagged task: assert its join rows go with it, verified by
      `gsd tags list` usage counts dropping.
- [ ] Assert tagging is unguarded: tag a task governed by an archived area
      and a task in a resolved project; both succeed.
- [ ] Assert bare `gsd tags` is a usage error with exit code 2 and no
      database side effect.
- [ ] Inspect human output for the tags name/count table, one `Tags` row on
      `show`, one `Tagged:` mutation line, and the deletion line with its
      detached count (no ANSI styling).
- [ ] Run `make check` and report the command transcript and final clean
      result.
