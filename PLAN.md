# Milestone 1 — Capture loop implementation plan

## Outcome

Deliver the complete lifecycle of a bare inbox task: capture, list, inspect,
edit, complete, cancel, reopen, and delete. Every command supports human and
JSON output, stable structured errors, and the existing 0/1/2 exit-code
contract.

## Settled design

- `--json` controls the complete output mode. JSON mode envelopes every error;
  default mode keeps human-readable diagnostics.
- Stable error codes are `not_found`, `invalid_argument`, `conflict`, `usage`,
  and `internal`.
- Cobra commands adapt input and output, an internal task service owns use
  cases and transaction boundaries, and a concrete SQLite store owns schema
  and persistence.
- Lifecycle transitions are strict: `done` and `cancel` require an open task;
  `reopen` requires a resolved task. Invalid source states are conflicts.
- Notes use `TEXT NOT NULL DEFAULT ''`; `--note ""` clears a note and
  `--note -` reads stdin through EOF without stripping trailing newlines.
- Human tables use `charm.land/lipgloss/v2/table` without styling.
- Logging and color are out of scope. Full color behavior remains in
  Milestone 6.

## Data and command contracts

- Use `modernc.org/sqlite` and schema revision `9001`.
- Create the reduced `STRICT` `tasks` table and open-task `inbox` view from the
  milestone specification.
- Resolve the database path as `--db` > nonempty `GSD_DB` > XDG data path,
  falling back to `~/.local/share/gsd/gsd.db`.
- Bootstrap only a genuinely empty version-0 database. A nonempty or
  differently versioned database fails with `conflict` and delete-your-dev-db
  guidance.
- Enable foreign keys on every connection, use a short busy timeout, and do
  not enable WAL without a demonstrated need.
- Calculate a new task's position after the maximum position across all inbox
  tasks, including resolved tasks. Preserve position through lifecycle
  changes.
- Stamp UTC millisecond timestamps on creation, edits, and lifecycle changes.
  Delete returns the row as it existed.
- JSON entities contain every current table column, nullable timestamps use
  `null`, collections use arrays including `[]`, and `tags` is absent until
  Milestone 5.
- JSON output is one compact value plus a newline. Field names, types, and
  error codes are stable; field order and message wording are not.
- Human collections are headerless tables, `show` is a field/value table, and
  mutations produce concise action-prefixed stdout payloads. Empty collections
  print nothing.
- Positive decimal IDs, nonblank titles, and valid UTF-8 are required.
  Semantic validation failures use `invalid_argument` and exit 1; command
  syntax failures use `usage` and exit 2.

## Chunks

- [x] **Chunk 1 — Store and output spine**

  Update the reviewed milestone and command specifications before writing
  behavior. Add the SQLite dependency, database-path resolution, safe
  bootstrap, embedded reduced schema, revision guard, connection pragmas, and
  concrete store. Establish the task service and root wiring without opening a
  database for help or version. Add output-mode selection, compact JSON entity
  and error writers, human error formatting, and stable error-to-exit mapping.
  Deliver `add`, `inbox`, and `show` through the complete command → service →
  store path. Record the proven command/service/store/output pattern in
  `AGENTS.md` as the template for later commands.

- [ ] **Chunk 2 — Lifecycle verbs**

  Deliver `done`, `cancel`, `reopen`, `delete`, and
  `list --status open|done|cancelled|all`. Implement strict transitions with
  conditional writes so concurrent state changes cannot silently succeed.
  Keep each mutation atomic, preserve task positions, stamp lifecycle
  timestamps, and capture deleted rows inside the delete transaction. Emit the
  affected row in JSON mode and the approved human payload in default mode.
  Prove status filtering, position ordering, conflicts, not-found behavior,
  and deletion without prompting.

- [ ] **Chunk 3 — Editing, stdin, and complete presentation**

  Deliver atomic `edit` support for `--title` and `--note`, reject edits with no
  requested field, and implement `--note -` for both `add` and `edit`. Preserve
  accepted text exactly while rejecting blank titles and invalid UTF-8. Finish
  the plain Lip Gloss table renderers for collections and `show`, with no ANSI
  or logging dependency. Complete the subprocess e2e workflow, consolidate the
  approved specification changes, and ensure the documented user stories match
  shipped behavior.

## Test ownership

- Store/application integration tests use real temporary SQLite databases.
  They protect the SQLite-specific schema, bootstrap guard, generated status,
  transaction, and conditional-transition behavior; this is the cheapest
  faithful layer because mocks cannot prove those boundaries.
- Focused command tests protect output streams, JSON envelopes, exit mapping,
  argument validation, and stdin handling. These contracts are owned at the
  adapter layer and do not need the full subprocess matrix.
- One subprocess e2e workflow protects complete binary wiring, persistent state
  across invocations, environment and flag handling, and stdout/stderr/exit
  behavior. It does not duplicate every integration-test case.

## Agent-verified end-to-end workflow

After all three chunks:

1. Run `make check`.
2. Run `make build` and use the built binary with a fresh database under
   `.sandbox/`, using `--json` throughout.
3. Add three tasks and capture their returned IDs.
4. Verify `inbox` returns exactly those tasks in position order.
5. Complete one task and cancel another; verify `inbox` has one task and
   `list --status all` has all three with correct statuses.
6. Reopen the completed task, complete it again, then repeat `done`; verify a
   `conflict` envelope on stderr and exit 1.
7. Edit a title and read a note from stdin; verify `show` preserves both.
8. Delete one task; verify the delete echo, then verify `show` returns
   `not_found` on stderr and exit 1.
9. Open a database with the wrong `user_version`; verify the throwaway-database
   conflict and exit 1.
10. Confirm JSON output contains no terminal control sequences and human empty
    lists produce no output.
11. Report the command transcript and results. Javier then runs the milestone's
    human user-story demo.
