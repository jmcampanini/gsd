# Milestone 10 — Query

Data mode: live. Depends on: Milestone 9.

Written light; re-review at plan gate (see note in `MILESTONE_8.md`).

## Capability

The escape hatch that makes gsd infinitely extendable without new
features: `gsd query` runs arbitrary read-only SQL against the documented
schema. This formally publishes `SCHEMA.md`'s query contract — tables
stable, views only gain columns — as user-facing API. v1 is complete at
the end of this milestone.

## Scope

- Command:

```text
gsd query "SELECT ..."
gsd query -              # SQL from stdin
```

- Read-only by construction (COMMANDS.md § Search and query): a separate
  read-only connection (`query_only` pragma and/or open-mode flag —
  both, belt and suspenders). Any write attempt fails as
  `invalid_argument`; so do non-SELECT statements.
- Human output: aligned table of selected columns; `--json`: array of
  row objects keyed by column name.
- Documentation: `SCHEMA.md`'s contract section gets a short "recipes"
  addendum (reverse tag lookup, per-project counts, area review) —
  documented example queries, explicitly not schema.

## Chunks

1. **The whole command** — read-only connection, output modes, error
   mapping, recipes doc. Single chunk.

## User stories

### Questions gsd never anticipated get answered anyway

```text
$ gsd query "SELECT governing_area_title, COUNT(*) c
             FROM available GROUP BY 1 ORDER BY c DESC"
Home   7
Work   4
```

### Agents get the whole database, safely

```text
$ echo "SELECT id, title FROM logbook LIMIT 3" | gsd query - --json
[{"id": 41, "title": "..."}, ...]
$ gsd query "DELETE FROM tasks"
{"error": {"code": "invalid_argument", "message": "query is read-only"}}
```

## Agent-verified end-to-end workflow

On a copy of the live database:

1. Every documented recipe runs and returns plausibly-shaped results.
2. Write-attempt matrix: `DELETE`, `UPDATE`, `INSERT`, `PRAGMA
   user_version = 9`, `ATTACH` — all refused, database file unchanged
   (checksum before/after).
3. The three contract views expose exactly the columns `SCHEMA.md`
   promises (introspection check).
4. Capstone: the agent answers a real question about Javier's tasks
   ("what did I finish last week, by area?") using only `gsd query`.

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] v1 retrospective: remaining wishes filed as post-v1 candidates
      (TUI, recurrence, Today — see `OVERVIEW.md` § "Deferred to v2+"),
      map closed out.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003, CLI-DOCS-004.
