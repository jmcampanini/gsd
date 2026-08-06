# Milestone 9 — Query

Data mode: throwaway. Depends on: Milestone 8.

Written light; re-review at plan gate.

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

## Carried from Milestone 8

The Milestone 8 foundation review's fix-now findings were resolved in
its wrap-up; there is no chunk 0 work. The deliberately deferred items
carry forward with their revisit triggers:

- **Config report generalization** — on config key #2 (`[serve] addr`,
  arriving in Milestone 11): add source classification and tag-derived
  env/flag spellings to go-config-loader's `configreporter` so gsd's
  renderer becomes a generic provenance-row loop like the sibling CLIs,
  and revisit the reporting/redaction contract at the same moment. No
  load-request struct — positional load parameters are the family
  idiom.
- **Genericizing the intentionally-parallel tag service flows** —
  carried from Milestone 6: revisit on the first sibling-divergence bug
  or a post-v1 attach-semantics change.
- **Typed transition spec for `applyTransition`** — carried from
  Milestone 6: revisit if post-v1 work adds transitions.
- **`search.Hit` constructors and accessors** — the hand-rolled sum
  type's invariant (exactly one entity pointer, matching `Kind`) is
  enforced at its consumers: revisit on the first new `Hit` consumer or
  producer, expected at the TUI milestones.
- **Entity-plus-container-titles projection consolidation** — logbook
  entries, task views, and search hits each assemble container-title
  context concretely in parallel, per convention: revisit on the fourth
  projection or the first context-inconsistency bug between surfaces.
- **In-expression scoping operators** (`in:`, `is:`, `~stem`/trigram
  markers) — parked: revisit when unfiltered search proves too broad in
  daily use; the spellings are reserved by FTS5 rejection today, and
  the virtual index makes alternate tokenizers a per-invocation swap.
- **Embeddings / semantic search** — parked, post-v1: revisit if
  tag-based topical search (`--related`) proves insufficient in daily
  use; the realistic path is an optional local-encoder sidecar fused
  with FTS, and nothing in Search forecloses it.
- **bm25 weight tuning** — the 4/3/2/1 values are a starting point:
  revisit after real-data use; tests pin ordering properties only, so a
  retune is a one-line change.

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
