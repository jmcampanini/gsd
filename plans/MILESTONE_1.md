# Milestone 1 — Capture loop

Data mode: throwaway. Depends on: Milestone 0.

## Capability

The full lifecycle of a bare task in the inbox: capture, list, inspect,
edit, complete, cancel, reopen, delete. `--json` and structured errors on
every command from the start. At the end of this milestone gsd is a real
(if minimal) todo tool you can run an inbox with.

## Schema delta

`tasks` table, reduced to what exists so far — no containers, no dates:

- `id`, `title`, `note`, `done_at`, `cancelled_at`, `status` (generated),
  `position`, `created_at`, `updated_at`
- `CHECK (done_at IS NULL OR cancelled_at IS NULL)`; `STRICT`
- `inbox` view, reduced: open tasks (containment clauses arrive with the
  columns, Milestones 3–4)

Staging note: columns and CHECKs from `SCHEMA.md` that reference projects,
areas, or dates arrive in their own milestones; the table converges on the
spec by end of Milestone 5.

Database bootstrap: path precedence `--db` flag > nonempty `GSD_DB` env >
`$XDG_DATA_HOME/gsd/gsd.db`, falling back to
`~/.local/share/gsd/gsd.db`; parent directories created when the database is
opened. Every connection enables foreign keys and a short busy timeout; WAL
is not enabled without a demonstrated need. Schema revision `9001` bootstraps
only a genuinely empty version-0 database. An existing revision-9001 database
opens normally; a nonempty version-0 or differently versioned database fails
with `conflict` and delete-your-dev-db guidance.

## Commands

```text
gsd add "TITLE" [--note TEXT|-]
gsd inbox
gsd list [--status open|done|cancelled|all]   # default: open
gsd show N
gsd edit N [--title TEXT] [--note TEXT|-]
gsd done N
gsd cancel N
gsd reopen N
gsd delete N
```

Output contract from day one (COMMANDS.md § Output contract): global
`--json` controls the complete output mode; mutations echo the affected entity
as its row, including derived `status`; every JSON error is a compact
`{"error":{"code":...,"message":...}}` value on stderr; exit codes are
0/1/2. Stable error codes are `not_found`, `invalid_argument`, `conflict`,
`usage`, and `internal`. Human collections and `show` use unstyled
`charm.land/lipgloss/v2/table` tables; mutations use concise action-prefixed
stdout payloads; empty collections print nothing. Logging and color are out of
scope until Milestone 6.

Positive decimal IDs, nonblank titles, and valid UTF-8 are required. Semantic
validation failures are `invalid_argument` with exit 1; syntax failures are
`usage` with exit 2. JSON entities contain every current table column,
nullable timestamps are `null`, collections are arrays including `[]`, and
`tags` remains absent until Milestone 5. JSON is one compact value plus a
newline; field order and message wording are not stable.

## Lifecycle and data defaults

- `done`/`cancel` on an already-resolved task, and `reopen` on an open task,
  are `conflict` errors — state transitions are explicit, no toggling.
- `delete` does not prompt (non-interactive contract, CLI-CMD-004); it echoes
  the deleted row as it existed.
- UTC millisecond timestamps are stamped on creation and every mutation.
- `position` append follows the maximum position across all inbox tasks,
  including resolved tasks; lifecycle changes preserve it.
- `--note ""` clears a note; `--note -` reads stdin through EOF without
  stripping trailing newlines.
- `tags` is absent from JSON output until tags exist (Milestone 5 adds it as an
  additive field).

## Chunks

1. **Store + output spine** — `internal/store` (open, bootstrap, schema,
   user_version guard), `--json` plumbing, error envelope, exit-code
   mapping, `add` / `inbox` / `show`. The architectural chunk: this review
   sets the pattern every later command copies.
2. **Lifecycle verbs** — `done`, `cancel`, `reopen`, `delete`,
   `list --status`, `conflict` semantics, e2e workflow test.
3. **Editing** — `edit` (`--title`, `--note`), `--note -` reading stdin
   for both `add` and `edit`, human output polish.

## User stories

### You can capture a thought before it escapes

```text
$ gsd add "Renew passport"
Added task 1: Renew passport
$ gsd inbox
  1  Renew passport
```

### You can work the inbox down and see where things went

```text
$ gsd done 1
Done: 1  Renew passport
$ gsd list --status done
  1  Renew passport   done
$ gsd reopen 1 && gsd cancel 1
```

### An agent can drive it without scraping

```text
$ gsd add "From a script" --json
{"id":2,"title":"From a script","note":"","done_at":null,"cancelled_at":null,"status":"open","position":1,"created_at":"...","updated_at":"..."}
$ gsd show 99 --json; echo $status
{"error":{"code":"not_found","message":"no task 99"}}
1
```

## Agent-verified end-to-end workflow

Against a fresh temp `--db`, using `--json` throughout:

1. `add` three tasks; capture IDs from the echoes.
2. `inbox` lists exactly those three, position order.
3. `done` one, `cancel` one; `inbox` shows one; `list --status all`
   shows three with correct statuses.
4. `reopen` the done one; `done` it again; `done` it a third time —
   expect `conflict`, exit 1.
5. `edit` a title and a note via stdin; `show` reflects both.
6. `delete` one; `show` it — expect `not_found`, exit 1.
7. Point the binary at a db with a wrong `user_version` — expect the
   throwaway-db error, exit 1.

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] Chunk 1's store/output patterns written into `AGENTS.md` as the
      template for all future commands.

## Standards

CLI-CMD-002/003/004, CLI-OUTPUT-002/003, CLI-DOCS-004.
