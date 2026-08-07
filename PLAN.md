# Milestone 9 — Go live: root implementation plan

[`plans/MILESTONE_9.md`](plans/MILESTONE_9.md) is authoritative for this
milestone's outcomes and acceptance boundary;
[`plans/PROCESS.md`](plans/PROCESS.md) governs workflow, review, and
consolidation. This plan is temporary and is retired at consolidation.

## Progress

- [x] Chunk 1 — Migration runner

There is no chunk 0: the Milestone 8 foundation review's fix-now findings
were resolved in its wrap-up; the deferred items carry forward in
`MILESTONE_9.md` with their revisit triggers.

## Settled design

- **Layout.** Migrations live in `internal/store/migrations/NNNN_name.sql`,
  embedded via `embed.FS`. Files contain top-level `CREATE`, `ALTER`, and
  `DROP` DDL only; catalog loading rejects every other top-level statement
  before the database is opened, and temporary-schema changes are refused
  before revision stamping. The runner derives the max known revision from
  the filenames and stamps `PRAGMA user_version`
  itself after each migration, inside that migration's transaction —
  `0001_baseline.sql` therefore carries no `PRAGMA user_version` line.
- **Baseline.** `0001_baseline.sql` is the accumulated schema through
  Search: content identical to `SCHEMA.md`'s SQL blocks, statement order
  matched to the spec (indexes placed per-table). The embedded
  `schema.sql` and `bootstrap()` are replaced wholesale.
- **Database identity.** `PRAGMA application_id = 1196639281` identifies
  gsd databases. The baseline stamps it in the migration transaction
  before the revision stamp. A version-zero database with the default
  zero identity proceeds to the empty-database guard; every other
  identity mismatch is refused before revision checks or DDL with
  `database does not belong to gsd`.
- **Guard ladder on open** (all refusals `conflict`-coded, exit 1):
  - `user_version == max known` → open normally.
  - `0 < user_version < max known` → apply pending migrations in
    sequence, then open.
  - `user_version == 0`, database empty → apply the full chain from
    `0001`.
  - `user_version == 0`, database nonempty → refuse as a foreign SQLite
    file; message unchanged: `database is not empty; delete your
    development database and try again`.
  - `user_version > max known` → refuse: `gsd is older than this
    database (database revision N, this gsd supports up to M); upgrade
    gsd`.
- **No dev-range handling.** The 9000-range throwaway stamps are never
  met in the wild; a leftover falls into the newer-database refusal. Any
  canonical-doc claim that a dev-range guard exists retires at
  consolidation.
- **Failure semantics.** Each migration runs in its own `BEGIN IMMEDIATE`
  transaction; a mid-migration failure rolls back that migration only,
  leaving the database at the last good revision, and the next open
  resumes. Concurrent opens serialize on the immediate transaction plus
  the existing 5s busy timeout.
- **Stability contract, codified.** Additive-or-full-delete, enforced by
  a store-layer lint test that applies the chain migration by migration
  and diffs end states: an object surviving a migration keeps its
  SQLite-exposed column metadata (name, type, nullability, default,
  primary-key position, and generated-column kind) and associated foreign
  keys and uniqueness constraints exactly, with additions appended only;
  whole-object drops are legal; explicit indexes are exempt. The lint uses
  structured SQLite metadata and does not parse raw `CREATE` SQL for
  `CHECK` text, collations outside uniqueness constraints, foreign-key
  deferral mode, or generated expressions. `SCHEMA.md`'s contract section
  is amended to match at
  consolidation.
- **Test seam.** The runner's core takes a migrations slice; `Open`
  wires the embedded set; tests feed fabricated chains (multi-step,
  failing, contract-violating). `Open`'s signature and `cmd` wiring are
  unchanged.
- **Deferred, recorded:** the contract's durable prose home after
  `plans/` retires (the lint test carries the mechanical part
  regardless; `AGENTS.md` is the natural candidate).

## Chunk 1 — Migration runner

Human outcome: the database becomes durable — a fresh database is
created at the baseline revision silently, real data survives upgrades
by design, and a database this binary cannot understand is refused
loudly instead of being touched.

Implementation:

- [x] `internal/store/migrations/0001_baseline.sql`: the accumulated
      schema, statement order matched to `SCHEMA.md`, no `user_version`
      stamp; delete `schema.sql`.
- [x] `internal/store/migrate.go`: runner replacing `bootstrap()` — the
      identity and guard ladder above, sequential apply of pending
      migrations, one `BEGIN IMMEDIATE` transaction per migration with
      the `application_id` and `user_version` stamps inside it, max
      revision parsed from the embedded filenames; the core takes a
      migrations slice and `Open` wires the embedded set.
- [x] Refusal errors `conflict`-coded with the exact messages above;
      messages state semantics only, per the store-boundary contract.
- [x] Update revision expectations across existing store and e2e tests
      (9006 → 1) and rework the wrong-revision fixtures to the new
      ladder.

Verification (primary owner: store tests on real temp SQLite,
fabricated chains where the real chain is too short):

- [x] Fresh empty database: the full chain applies, lands at max
      revision, schema objects present.
- [x] Sequential apply: a database at revision k applies only k+1..max
      (fabricated two-migration chain).
- [x] Mid-migration failure: a failing fabricated migration rolls back
      atomically — schema and `user_version` both remain at the last
      good revision; a rerun succeeds once the chain is fixed.
- [x] Future revision: refused, `conflict`, exact message.
- [x] Foreign database at a supported revision: refused before DDL,
      `conflict`, file unchanged.
- [x] Nonempty version-0: refused, `conflict`, message unchanged.
- [x] Contract lint over the real embedded chain (end-state diff per
      migration, additive-or-full-delete, indexes exempt), plus
      fabricated violating chains — a column retype, a column loss, and
      a view recreation that loses a column — proving the lint fails
      them.
- [x] e2e (subprocess, inside `make check`): a fresh database opens at
      revision 1 (read from the file) and data persists across separate
      binary invocations; a stamped future revision is refused with the
      exact message and exit 1.
- [x] `make check` green.

Informational timing (not a gate):

- [x] Build the real binary and a same-source comparison binary under
      `.sandbox/` with only the migration check bypassed. Use `hyperfine`
      with warmups and repeated runs against an already-current database
      for read-only `inbox` and `show` commands. Report absolute latency,
      delta, and ratio; commit no benchmark code or target.

Human proof (chunk demo `.sandbox/demos/9-chunk-1.html`), exact
commands:

```sh
gsd --db .sandbox/demo.db add "file the taxes"   # fresh db: baseline applied silently
gsd --db .sandbox/demo.db inbox                  # new invocation: data persisted
sqlite3 .sandbox/demo-future.db "PRAGMA application_id = 1196639281; PRAGMA user_version = 2"
gsd --db .sandbox/demo-future.db inbox           # refused: gsd is older than this database
```

- [x] Agent verification before review: build the real binary, run the
      informational timing phase, run the demo command list against a
      fresh temporary database, capture the verbatim output into the deck,
      and pass local `make check`; then open the chunk PR against the
      milestone branch.

## Agent-verified end-to-end workflow

Run against the real built binary after the chunk merges
(`PROCESS.md` step 4); the equivalent durable subprocess coverage lives
in `e2e/` inside `make check`.

1. `make check` — includes the migration, lint, and e2e coverage.
2. With a fresh temporary database:
   1. `gsd add "first live task"` — creates the database and applies
      the baseline silently.
   2. `gsd inbox` in a new invocation — the task is present, and
      `PRAGMA user_version` on the file reads 1.
   3. Stamp a separate file with gsd's `application_id` and
      `user_version = 2`; any behavioral command against it fails with
      exit 1 and exactly `gsd is older than this database (database
      revision 2, this gsd supports up to 1); upgrade gsd`.
   4. Create a nonempty version-0 SQLite file (any foreign table); a
      behavioral command against it fails with exit 1 and the existing
      not-empty message.
   5. Create a foreign SQLite file at `user_version = 1`; a behavioral
      command fails with exit 1 and `database does not belong to gsd`,
      and the file remains byte-for-byte unchanged.
3. Report the clean transcript.

## Consolidation checklist (`PROCESS.md`)

- [x] `COMMANDS.md` § Database reconciled to live-era semantics with the
      exact refusal messages; no canonical document still claims a
      dev-range guard exists.
- [x] `SCHEMA.md` § stability contract amended to
      additive-or-full-delete: surviving objects gain appended columns
      only, removal is whole-object, indexes exempt, the lint test named
      as the enforcement.
- [x] `MILESTONES.md` data policy checked so the dev-range description
      reads as history only.
- [x] `DIVERGENCES.md` stays empty.
- [ ] Standard exit workflow plus `MILESTONE_9.md`'s exit criteria.
