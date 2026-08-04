# Milestone 7 — Root Implementation Plan

Reorder: manual ordering for tasks, projects, and areas. This is the
authoritative implementation and verification plan for the active milestone
under [`plans/PROCESS.md`](plans/PROCESS.md);
[`plans/MILESTONE_7.md`](plans/MILESTONE_7.md) owns the outcomes and
acceptance boundary. Both artifacts are retired at consolidation.

## Progress

- [ ] Chunk 0 — Milestone 6 consolidation
- [ ] Chunk 1 — The reorder verb

## Settled contract deltas

Decisions from the 2026-08-04 planning interview. No schema change;
`user_version` stays `9006`.

- **JSON echo is the bare entity row.** The milestone's proposed
  `{"task": {...}, "container": [ids]}` envelope is rejected: `reorder
  --json` returns the complete entity row (including `tags`), byte-identical
  in shape to `edit --json`, keeping every simple mutation uniform. Agents
  wanting the resulting order follow with `list`/`inbox`/`projects
  list`/`areas list`, which are position-ordered. Consolidation folds
  reorder into COMMANDS.md's "mutations echo the affected entity" rule and
  reconciles the milestone's proposed echo; no divergence entry — this is a
  spec-silent extension settled at plan time.
- **Reordering is status-blind.** Done/cancelled tasks and projects and
  archived areas can be reordered and can serve as `--after`/`--before`
  references; renumbering covers every row in the container regardless of
  status. Positions are container-scoped, not status-scoped (SCHEMA.md: an
  archived area keeps its position).
- **Renumbering is contiguous and 0-based**, matching the append formula
  `COALESCE(MAX(position), -1) + 1`. `updated_at` bumps only on the moved
  entity; sibling rows get their `position` rewritten silently.
- **No-op reorders succeed** (already first under `--first`, `--after` the
  current predecessor, single-element container): exit 0, normal echo,
  treated as ordinary reorders — the renumber runs and the moved entity's
  `updated_at` bumps.
- **Error package.** Flag-arity failures (none or several of
  `--after/--before/--first/--last`) are usage, exit 2, enforced by Cobra.
  Store-side, in one transaction, in precedence order: unknown N →
  `not_found` ("no task 9"); unknown M → `not_found`; self-reference →
  `invalid_argument` ("cannot reorder task 9 relative to itself");
  cross-container → `invalid_argument` ("task 12 is in a different
  container"). Codes are stable; wording is not.
- **Human output is a `~` mutation line** — `~ Reordered: <id>  <title>`,
  with the noun inserted for project/area variants — per the canonical
  styling grammar (`~` neutral mutations). The milestone user story's
  silent reorder is abridged; canonical outranks the deliberately lighter
  milestone file.

## Architecture decisions

- **Reorder mirrors `edit` end to end.** cmd adapts arguments/flags and
  presentation; each service validates shape (positive IDs via
  `domain.ValidateID`, positive reference when the placement is relative)
  and calls a new store `Reorder`; the store owns atomicity for the single
  operation via the existing `runInTransaction` machinery
  (`BEGIN IMMEDIATE`), performing existence, self-reference, and
  sibling checks inside the transaction, exactly like `store.Tasks.Edit`.
- **Placement is shared data; flows stay concretely parallel.** A single
  placement value type (anchor kind first/last/after/before plus reference
  ID) is pure data shared across nouns, following the `domain.TagNames`
  precedent. The three service flows, store methods, and cmd factories
  remain byte-parallel modulo entity nouns, matching the tagging flows.
- **Renumber inside the store core**: read the container's sibling IDs
  ordered by `position, id` (reusing the existing container predicates —
  `taskContainerOf` for tasks, `area_id IS ?` scoping for projects, the
  global list for areas), splice the moved ID to its target index in Go,
  then apply one CASE-based `UPDATE` that rewrites positions and bumps
  `updated_at` on the moved row only, and re-read the moved row
  tag-enriched through the existing `Find`. No unique constraint exists on
  `(container, position)`, so no two-phase shuffle is needed.
- **cmd surface**: `gsd reorder N`, `gsd project reorder N`,
  `gsd area reorder N`, one positional ID, four placement flags with
  `MarkFlagsOneRequired` + `MarkFlagsMutuallyExclusive` so arity failures
  are Cobra parse errors (usage, exit 2). A `verbReordered` entry joins the
  mutation verb table (`~` glyph, no accent). JSON mode marshals the bare
  entity through the existing writers; no new writer machinery.
- **Help, version, and argument parsing** continue to open neither the
  database nor the config file; the injected application factory is
  unchanged.

## Chunk 0 — Milestone 6 consolidation

Review-only (no new human-facing capability): applies the scheduled finding
from the Milestone 6 foundation review before Reorder work begins.
Behavior-preserving. No demo.

- [ ] Service-owned filtered-list validation for projects, mirroring the
      task sibling: `AreaExists` joins `project.Transaction`; the project
      store gains a `WithinReadTransaction` path; the area-existence check
      moves from the store's `listArea` into the project service's `List`;
      `listArea` drops its leading `findArea`.
- [ ] Store tests pinning the unknown-area error migrate to project service
      tests, mirroring the task side's tag-resolution migration.
- [ ] Verification: full `make check`; envelopes, exit codes, and error
      messages unchanged.

The deliberately deferred items (config report generalization, genericizing
the parallel tag flows, typed transition spec) stay recorded in
`plans/MILESTONE_7.md` with their revisit triggers; chunk 0 does not touch
them.

## Chunk 1 — The reorder verb

Human outcome: manual ordering — move any task, project, or area anywhere
within its container and watch every list obey.

- [ ] Shared placement value type in `internal/domain`: anchor kind
      (`first`, `last`, `after`, `before`) plus reference ID; validation
      helper for shape.
- [ ] Services: `Reorder(ctx, id, placement)` on task, project, and area
      services — shape validation only, then delegate to the store;
      concretely parallel across the three nouns.
- [ ] Stores: `Reorder` on `store.Tasks`, `store.Projects`, `store.Areas` —
      pooled wrapper plus core method inside `runInTransaction`
      (`BEGIN IMMEDIATE`): find N, find M when relative, self-reference and
      same-container checks, status-blind contiguous 0-based renumber via
      one CASE `UPDATE`, `updated_at` bump on the moved row only,
      tag-enriched re-read of the moved row.
- [ ] cmd: `gsd reorder N`, `gsd project reorder N`, `gsd area reorder N`
      with the four placement flags (one required, mutually exclusive),
      `verbReordered` mutation line, bare-entity JSON echo, factories
      byte-parallel with the tagging commands.
- [ ] Test owners — store tests on real temporary SQLite own the semantics:
      the placement matrix per noun, contiguous renumbering, status-blind
      membership, moved-row-only `updated_at`, no-op success, error codes
      and precedence (unknown N, unknown M, self-reference,
      cross-container), atomicity. Service tests with store fakes own shape
      validation and error passthrough. cmd tests own flag-arity → exit 2,
      the bare-entity JSON envelope, the `~ Reordered:` human line, stream
      routing, and exit mapping.
- [ ] e2e: new `e2e/reorder_test.go` owns the milestone workflow against
      the built binary — reorder matrix per noun asserting full container
      ordering after each placement, cross-container and self-reference
      errors, and mixed-operation stability (add, done, reorder, delete
      interleaved; ordering consistent, no position collisions).
- [ ] Human proof (demo `.sandbox/demos/milestone-7-chunk-1.html`), against
      a fresh temp db seeded with:
      `gsd add "Call plumber"` (task 1); `gsd add "Renew passport"`
      (task 2); `gsd add "Buy milk"` (task 3);
      `gsd projects add "Kitchen reno"` (project 1);
      `gsd projects add "Garden prep"` (project 2);
      `gsd add "Order tiles" --project 1` (task 4);
      `gsd areas add "Home"` (area 1); `gsd areas add "Work"` (area 2);
      `gsd areas add "Errands"` (area 3). Slides:
      1. `gsd inbox` — the seeded baseline (1, 2, 3);
      2. `gsd reorder 2 --first` then `gsd inbox` — the important thing
         sits on top (2, 1, 3);
      3. `gsd reorder 1 --last` then `gsd inbox` — (2, 3, 1);
      4. `gsd reorder 3 --before 2` then `gsd inbox` — (3, 2, 1);
      5. `gsd reorder 1 --after 3` then `gsd inbox` — (3, 1, 2);
      6. `gsd reorder 3 --after 4` — order is honest about its scope:
         `invalid_argument`, task 4 lives in a project;
      7. `gsd projects list`, `gsd project reorder 2 --first`,
         `gsd projects list` — (2, 1);
      8. `gsd areas list`, `gsd area reorder 3 --before 1`,
         `gsd areas list` — (3, 1, 2).
- [ ] Agent verification: `make check` green; the full placement matrix,
      error cases, and mixed-operation workflow exercised via the built
      binary.

## Agent-verified end-to-end workflow

Run against the real built binary after all chunks merge; the durable
equivalent lives in `e2e/reorder_test.go` inside `make check`.

1. Fresh temp db seeded across all three container kinds (inbox tasks,
   project tasks, area tasks; standalone and area projects; multiple
   areas).
2. Reorder matrix per noun: `--first`, `--last`, `--after`, `--before`;
   assert the full container ordering after each move via the
   position-ordered collection commands.
3. Cross-container and self-reference errors surface `invalid_argument`;
   unknown IDs surface `not_found`; flag-arity failures exit 2 without
   touching the database.
4. Mixed-operation stability: add, done, reorder, and delete interleaved in
   one container; ordering stays consistent and JSON `position` values stay
   collision-free and contiguous.
5. JSON echo is the bare entity row; a follow-up position-ordered `list`
   confirms the new order in the same shapes agents already consume.
