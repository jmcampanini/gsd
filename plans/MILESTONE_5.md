# Milestone 5 — Tags

Data mode: throwaway. Depends on: Milestone 4.

## Capability

Cross-cutting labels over the whole graph: one flat, case-insensitive
namespace attaching to tasks, projects, and areas. With this milestone the
v1 entity model — and therefore the schema `SCHEMA.md` promises — is
complete.

## Schema delta

- `tags` table (`UNIQUE COLLATE NOCASE`) and the three join tables
  (`task_tags`, `project_tags`, `area_tags`), composite PKs,
  `WITHOUT ROWID`, CASCADE both sides, reverse-lookup indexes — exactly
  per `SCHEMA.md`.
- Views rebuilt with the `tags` JSON-array enrichment column. After this
  milestone the schema must be byte-comparable to `SCHEMA.md` (see exit
  criteria).
- `user_version` bump — the last one; this becomes the schema baseline used
  at Go live.

## Commands

```text
gsd tags add NAME
gsd tags list                 # includes usage counts
gsd tags rename OLD NEW
gsd tags delete NAME          # detaches from everything
gsd tag N NAME...             # task; singular noun forms for others:
gsd untag N NAME...
gsd project tag N NAME...     / gsd project untag N NAME...
gsd area tag N NAME...        / gsd area untag N NAME...
gsd add "TITLE" [--tag NAME]... ;  projects add / areas add likewise
gsd list [--tag NAME] ...
```

## Semantics being proven here

- Tags are addressed by name, never ID; names are the identity.
- Tags must pre-exist: `tag`/`untag`/`--tag` with an unknown name is an
  error (`not_found`) — `gsd tags add` is the only creation path.
- Case-insensitive uniqueness uses SQLite `NOCASE`, which folds ASCII only:
  `Errands` vs `errands` is a `conflict`, while non-ASCII case variants remain
  distinct. Attach/detach/filter use the same matching. Successful human
  mutation lines use stored spelling after that resolution, including both
  names in a rename.
- Deleting a tag detaches it everywhere (join-row CASCADE), reported by
  count. Entity JSON now includes `tags` as an array of names — the
  output contract's final field.
- **Proposed defaults**: duplicate tagging is a no-op success (idempotent
  attach); `tags list` counts usage across all three entity kinds and
  displays alphabetically (per `SCHEMA.md`, tags carry no position).

## Chunks

0. **Transaction seam groundwork** — the `Tasks` store gains the
   executor + `WithinTransaction` seam before any tag behavior lands
   (see below).
1. **Tag administration** — `tags` table + joins, `tags add/list/
   rename/delete`, uniqueness and rename semantics.
2. **Attachment everywhere** — `tag`/`untag` verbs on all three nouns,
   `--tag` on the three `add` commands and `list` filter, view + JSON
   enrichment.

### Chunk 0: the Tasks store transaction seam

`--tag` on `add` turns task creation into a multi-statement write:
insert the task, insert its join rows, and fail with `not_found` when a
tag name does not exist — atomically. A single statement no longer
proves the complete operation, so per `AGENTS.md` this becomes a
service-owned transaction. Two gaps in `internal/store` stand in the
way:

- `Tasks` is the only store without the transaction seam: `Areas` and
  `Projects` take an executor interface plus `WithinTransaction`, while
  `Tasks` holds the raw database handle directly.
- The failure-classification reads (`findContainerState`, which
  re-queries project and area state to name blockers) construct sibling
  stores on the raw handle. Inside a transaction they would read outside
  it, classifying against state the transaction cannot see.

Chunk 0 closes both gaps before any tag behavior lands: give `Tasks`
the same executor + `WithinTransaction` shape as its siblings and route
the classification reads through the executor. No behavior changes; the
existing store and service suites pass untouched.

Chunk 0 also decides how the archived-area and open-project guards live
inside the new transaction, applied consistently to every task mutation
the milestone touches. Viable options:

- **Keep the guards in the statement.** The guarded
  `INSERT/UPDATE ... WHERE ... RETURNING` runs as the transaction's
  first statement; join-row inserts follow. Smallest diff and keeps the
  established idiom, but retains the dual maintenance between the SQL
  guard clauses and the Go classification pass, and the guard
  statements keep growing with each new rule. If this option wins, the
  repeated guard clauses (open-project, governing-area-active) are
  extracted into named SQL fragment constants written once and shared
  across the verbs, so each rule has a single source even though the
  classification pass remains.
- **Read-then-validate inside the transaction.** Select the container
  state under `BEGIN IMMEDIATE`, validate in Go with exact typed
  errors, then execute plain writes. Collapses the guard/classifier
  duplication and shrinks the oversized statements, but departs from
  the one-atomic-statement idiom for these verbs and is a materially
  larger refactor. `AGENTS.md` already permits it once the operation is
  genuinely multi-statement.

The acid test for the choice is `Tasks.Edit`: a ~175-line method whose
membership guard is assembled by string concatenation around a CTE
that exists only when membership changes, referenced by SQL fragments
whose reachability is enforced by an early return far from their use,
with argument splicing that fails only at bind time. Read-then-validate
dissolves all of that; keeping statement guards means wrapping it in a
transaction and living with its growth. Whichever way the decision
goes, it must be defensible against this method in particular.

Whichever option wins, chunk 0 also consolidates the duplicated
classification stanzas in the `Projects` store: `Resolve` and `Reopen`
carry near-identical find-row → find-area → archived-conflict blocks,
and `Edit` re-implements the singular/plural blocker message that
`blockers.go` already owns. Under read-then-validate they dissolve into
the in-transaction validation; under statement guards they collapse
into one shared classification helper.

Chunk 0 also retires the sentinel-row idiom in the contained listings
(`Tasks.listContained` and the area-scoped project listing), which
distinguishes a missing container from an empty one by UNION-ing a
fabricated all-zero row and filtering `ID != 0` in Go — an unstated
real-IDs-start-at-1 invariant plus a placeholder column list that must
track the real column lists by hand. With the transaction seam in
place, each listing becomes a container `Find` followed by the list
query inside one read transaction: same missing-versus-empty semantics
and snapshot consistency, with both the SQL and the Go scan paths
simplified.

Finally, chunk 0 extracts the service-layer helpers triplicated across
`task`, `project`, and `area` — title/note validation, positive-ID
checks, digit-strict ID parsing, and timestamp formatting — into one
shared package (e.g. `internal/domain`), parameterized by entity noun
so every message stays byte-identical. Without this, the `tag` package
mints copy number four, and a cross-entity rule change (trim titles,
cap lengths, change timestamp precision) needs N coordinated edits
with silent divergence when one is missed.

## User stories

### One label crosses every container

```text
$ gsd tags add errands
$ gsd tag 7 errands              # task
$ gsd project tag 2 errands
$ gsd list --tag errands
  7  Drop off dry cleaning
```

### The namespace stays clean

```text
$ gsd tags add Errands
{"error": {"code": "conflict", "message": "tag already exists: errands"}}
$ gsd tags rename errands out-and-about
$ gsd tags delete out-and-about
Deleted tag out-and-about (detached from 3 items)
```

### Unknown tags never spring into existence

```text
$ gsd tag 7 groceries
{"error": {"code": "not_found", "message": "no tag groceries"}}
```

## Agent-verified end-to-end workflow

Fresh temp db, `--json`:

1. Create tags; attach one tag to a task, a project, and an area;
   `tags list` shows usage count 3.
2. Unknown-name matrix: `tag`, `untag`, `add --tag` → `not_found`.
3. Case-insensitivity: create `Errands` → conflict with `errands`;
   `tag N ERRANDS` attaches the existing tag.
4. Duplicate attach → success, join row count unchanged.
5. `rename` propagates: entity JSON `tags` arrays show the new name.
6. `tags delete` → subsequent `show` on all three entities has the tag
   gone; entities themselves intact.
7. Delete a tagged task → its join rows go with it, verified by
   `tags list` usage counts dropping.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] **Schema convergence audit**: dump the dev database schema and diff
      against `SCHEMA.md`'s DDL (tables, checks, indexes, views). Any
      difference is either fixed or recorded temporarily in `DIVERGENCES.md`,
      then reconciled and removed before Config begins. This diff proves that
      the staged construction arrived at the spec.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003.
