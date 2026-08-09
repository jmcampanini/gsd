# Milestone 17 — Query

Data mode: **live**. Depends on: Milestone 9 (schema stability
contract). **Optional**: returns only if daily use demonstrates the
need for raw SQL; sequenced after Serve (Milestone 16).

Written light; re-review at plan gate on activation.

## Capability

The escape hatch that makes gsd infinitely extendable without new
features: `gsd query` runs arbitrary read-only SQL against the
documented schema, exposing the stability contract `SCHEMA.md` has
carried since Go live — tables stable, views only gain columns — as a
user-facing query surface.

## Scope

- Command:

```text
gsd query "SELECT ..."
gsd query -              # SQL from stdin
```

- Read-only by construction: a separate read-only connection
  (`query_only` pragma and/or open-mode flag — both, belt and
  suspenders). Any write attempt fails as `invalid_argument`; so do
  non-SELECT statements.
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

On a seeded temporary database:

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

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)).
