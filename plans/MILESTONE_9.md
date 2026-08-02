# Milestone 9 — Query

Data mode: throwaway. Depends on: Milestone 8.

Written light; re-review at plan gate (see note in `MILESTONE_7.md`).

## Capability

The escape hatch that makes gsd infinitely extendable without new
features: `gsd query` runs arbitrary read-only SQL against the documented
schema. This formally publishes `SCHEMA.md`'s query contract — tables
stable, views only gain columns — as user-facing API. The v1 functional
surface is complete at the end of this milestone; Go live then turns
gsd into the system of record.

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

On a seeded dev database:

1. Every documented recipe runs and returns plausibly-shaped results.
2. Write-attempt matrix: `DELETE`, `UPDATE`, `INSERT`, `PRAGMA
   user_version = 9`, `ATTACH` — all refused, database file unchanged
   (checksum before/after).
3. The three contract views expose exactly the columns `SCHEMA.md`
   promises (introspection check).
4. Capstone: the agent answers a nontrivial question about the seeded
   data ("what was finished last week, by area?") using only
   `gsd query`.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] **Functional completeness audit**: every command and behavior
      `COMMANDS.md` promises (excluding the forward-looking TUI section)
      is demonstrated by the accumulated e2e and agent-verified
      workflows; each gap is fixed or explicitly re-scoped before Go
      live begins.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003, CLI-DOCS-004.
