# Milestone 8 — Root Implementation Plan

Search: FTS5 full-text search over tasks, projects, and areas. This is the
authoritative implementation and verification plan for the active milestone
under [`plans/PROCESS.md`](plans/PROCESS.md);
[`plans/MILESTONE_8.md`](plans/MILESTONE_8.md) owns the outcomes and
acceptance boundary. Both artifacts are retired at consolidation.

## Progress

- [ ] Chunk 1 — Direct search
- [ ] Chunk 2 — Related search

There is no chunk 0: the Milestone 7 foundation review scheduled no
findings.

## Settled contract deltas

Decisions from the 2026-08-05 planning interview. They amend the milestone
file's lightly-written implementation sketch at its declared plan gate.

- **The index is virtual.** A `temp` FTS5 table is built per invocation
  from one live document-assembly query; nothing persists. There is no
  schema change: `user_version` stays `9006`, the milestone's proposed
  `9008` bump and "index schema" chunk are dropped, and there are no
  triggers, no sync code, no backfill, and no staleness class — every
  search derives from current data. Feasibility and cost are verified on
  `modernc.org/sqlite` v1.55.0: temp FTS5 tables, column filters, and
  weighted `bm25()` all work; measured 25ms to build 5,000 documents and
  2ms to query them. Consolidation reconciles the MILESTONES.md note that
  the go-live baseline "includes the FTS schema" — nothing persists, so
  v1 still ships no live migrations, now vacuously.
- **Filter flags are descoped.** `search` takes no `--project`, `--area`,
  `--tag`, or `--status`; it always spans all three kinds, all statuses,
  and archived areas. The milestone's second user story ("Search narrows
  like list does") is cut. Consolidation amends COMMANDS.md's search
  grammar line and its "composes with the `list` filter flags" bullet.
  Future scoping arrives, if ever, as in-expression operators (`in:`,
  `is:`, `~stem`); those spellings are malformed FTS5 today and fail
  `invalid_argument`, so the namespace stays naturally reserved.
- **Two modes over one document set.** Each entity's document has four
  weighted columns: `title`, `tags`, `note`, `context`. Context is
  inherited text — for a task, its project's title and tags plus its
  governing area's title and tags; for a project, its area's title and
  tags; for an area, nothing. Default (direct) search column-filters to
  `{title tags note}`; `--related` matches all four columns, pulling in
  contextual hits ranked below direct ones.
- **Results are relevance-ordered**: weighted `bm25` (title 4, tags 3,
  note 2, context 1), ties broken by kind (task, project, area), then
  `id` ascending. The weights are internal and tunable; tests assert
  ordering properties ("a title hit outranks a context-only hit"), never
  scores. Tokenizer is FTS5's default `unicode61`, no stemming — prefix
  syntax (`plumb*`) is the recall idiom, and the virtual index makes a
  future stemmed or trigram variant a per-invocation swap.
- **Error package.** A blank or whitespace expression is
  `invalid_argument` from the service before any database work
  (title-style UTF-8/nonblank validation with expression wording). A
  syntactically malformed expression is `invalid_argument` from the
  store, which maps SQLite's distinguishable `fts5: syntax error`; only
  the FTS parser can judge expression validity. Raw passthrough means
  FTS5 column-filter spellings against internal column names
  (`title:plumb*`) do whatever FTS5 does — unstable, undocumented
  behavior, deliberately unpinned.
- **JSON hit shape**: `{"kind":"task", ...}` — the `kind` discriminator
  plus the complete canonical entity row for that kind, `tags` included,
  flattened at top level. No score and no related/direct marker; array
  order carries relevance, and metadata fields remain addable later
  without breaking. Empty results are `[]`.
- **Human output** is an aligned table — `kind, id, title, status,
  context` — under the standard faint headers. Status shows task/project
  status and `archived` for archived areas (blank when active, matching
  `areas list`). Context is the faint container-title path
  (`Bathroom plumbing · Home`) so related hits self-explain and duplicate
  titles disambiguate; blank for inbox tasks. No snippets. Empty results
  print nothing.

## Architecture decisions

- **Search mirrors logbook end to end.** A new `internal/search` package
  owns a `Hit` type (kind, one populated entity row, context titles) and
  a `Service` over a narrow store interface. The service validates the
  expression (nonblank, valid UTF-8) and delegates; `store.Search` owns
  everything SQLite: temp-index lifecycle, document assembly, matching,
  ranking, and error mapping. cmd adapts the argument, the `--related`
  flag, and presentation.
- **Temp index lifecycle**: the store creates
  `temp.search_index` (`kind` and `entity_id` UNINDEXED; `title`, `tags`,
  `note`, `context`), populates it from one document-assembly statement
  whose joins compute inheritance live (tag titles aggregated per entity;
  container titles and tags composed into `context`), queries it, and
  drops it. The pool is capped at one connection, so the temp schema is
  stable within an invocation and vanishes at process exit.
- **Query shape**: direct mode wraps the user expression as
  `{title tags note} : (EXPR)`; related mode passes `EXPR` unfiltered.
  Both order by
  `bm25(search_index, 0, 0, 4.0, 3.0, 2.0, 1.0)`, then the kind CASE,
  then `entity_id`. The store then fetches complete tag-enriched entity
  rows per kind (reusing the existing column lists and row scanners) and
  reassembles them in match order.
- **cmd surface**: top-level `gsd search "EXPR"` with
  `cobra.ExactArgs(1)` and a `--related` boolean flag. Help, version, and
  argument parsing continue to open neither the database nor the config
  file; the injected application factory is unchanged. JSON marshals
  kind-discriminated flattened rows through the existing writers; human
  output adds one collection writer following the logbook/areas-list
  precedents (faint kind/id/context, `statusWord`, red `archived`).

## Chunk 1 — Direct search

Human outcome: `gsd search "EXPR"` finds any task, project, or area by
its own title, tags, or note — FTS5 syntax and all — ranked by
relevance, across every status and archived areas.

- [ ] `internal/search`: `Hit` (kind, entity row, context titles) and
      `Service.Search(ctx, expression)` — expression nonblank/UTF-8
      validation with expression-worded `invalid_argument`, then
      delegate to the store.
- [ ] `internal/store/search.go`: temp FTS5 index built per call from
      the document-assembly statement (all four columns, inheritance
      computed live — context content lands now even though direct
      matching ignores it); direct column-filtered MATCH; weighted bm25
      ordering with kind/id tie-breaks; `fts5: syntax error` →
      `invalid_argument`; full tag-enriched entity rows fetched per kind
      and returned in match order with context titles.
- [ ] cmd: `gsd search "EXPR"` command, kind-discriminated flattened
      JSON rows, human `kind, id, title, status, context` table.
- [ ] Test owners — store tests on real temporary SQLite own the
      semantics: document assembly per kind (own text and inherited
      context content, tag aggregation), match syntax passthrough
      (prefix, phrase, OR), ranking properties (title > tags > note;
      kind/id tie-breaks), inclusion of resolved tasks/projects and
      archived areas, malformed-expression mapping, repeated-search
      cleanliness, empty results. Service tests with a store fake own
      blank-expression rejection and error passthrough. cmd tests own
      the JSON hit shape, the human table (context column, `archived`
      status, faint styling), stream routing, exit mapping, and empty
      output (`[]` / nothing).
- [ ] Human proof (demo `.sandbox/demos/milestone-8-chunk-1.html`),
      against a fresh temp db seeded with:
      `gsd tags add house`; `gsd tags add reno`; `gsd tags add errands`;
      `gsd areas add "Home"` (area 1); `gsd area tag 1 house`;
      `gsd areas add "Cabin" --note "lake house projects"` (area 2);
      `gsd area archive 2`;
      `gsd projects add "Bathroom plumbing" --area 1` (project 1);
      `gsd project tag 1 reno`;
      `gsd add "Call plumber"` (task 1); `gsd tag 1 errands`;
      `gsd add "Buy pipe wrench" --area 1 --note "for the bathroom"`
      (task 2);
      `gsd add "Fix sink" --project 1` (task 3);
      `gsd add "Order tiles" --project 1` (task 4); `gsd done 4`.
      Slides:
      1. `gsd search "plumb*"` — task 1 and project 1: mixed kinds,
         kind beside id, `Home` in project 1's context column;
      2. `gsd search bathroom` — project 1 above task 2: a title hit
         outranks a note hit;
      3. `gsd search 'tile* OR errand*'` — task 4 (`done` status
         visible) and task 1 (tag match): OR passthrough, tags are
         searchable, resolved tasks included;
      4. `gsd search cabin` — area 2 with `archived` in the status
         column;
      5. `gsd search "plumb* AND"` — malformed expression:
         `invalid_argument`, exit 1, no panic;
      6. `gsd search "sink" --json` — the kind-discriminated complete
         entity row.
- [ ] Agent verification: `make check` green; the seeded searches,
      ranking property, error cases, and JSON shape exercised via the
      built binary.

## Chunk 2 — Related search

Human outcome: `--related` widens the same search through project, area,
and tag context — "Fix sink" surfaces for `plumb*` because of where it
lives and how its containers are tagged.

- [ ] Service and store: thread a `related` mode flag through
      `Search`; related mode matches all four columns unfiltered so
      context-only hits join the results below direct hits.
- [ ] cmd: `--related` flag wiring; output shapes unchanged.
- [ ] Test owners — store tests own related semantics: the context
      pull-out matrix (member task surfaces via project title, project
      tag, area title, and area tag; project surfaces via area title
      and tag), context-only hits ranked below direct hits, direct mode
      still excluding context-only hits, and freshness (rename a
      project or tag, re-search: reflected immediately with no
      persisted state to go stale). Service tests own mode passthrough.
      cmd tests own flag wiring and envelope stability across modes.
- [ ] Human proof (demo `.sandbox/demos/milestone-8-chunk-2.html`),
      against the chunk 1 seed. Slides:
      1. `gsd search "plumb*"` then `gsd search "plumb*" --related` —
         tasks 3 and 4 join below the direct hits, explained by their
         context column;
      2. `gsd search reno --related` — project 1 by its own tag; its
         tasks by inheritance;
      3. `gsd search house --related` — area 1 by its own tag;
         everything under Home by inheritance;
      4. `gsd project edit 1 --title "Bath remodel"` then
         `gsd search remodel --related` — the rename is searchable
         immediately: no reindex exists to lag;
      5. `gsd search "plumb*" --related` after the rename — tasks 3
         and 4 no longer surface through the old project title: context
         derives live.
- [ ] Agent verification: `make check` green; the pull-out matrix,
      mode contrast, and rename-freshness workflow exercised via the
      built binary.

## Agent-verified end-to-end workflow

Run against the real built binary after all chunks merge; the durable
equivalent lives in `e2e/search_test.go` inside `make check`.

1. Fresh temp db seeded across all three kinds with tags, an archived
   area, a resolved task, and both project-contained and loose tasks.
2. Direct mode: prefix, phrase, and OR expressions return the expected
   known-data hits across kinds; a title hit ranks above a note-only
   hit; resolved and archived entities appear; blank and malformed
   expressions are `invalid_argument` (exit 1), flag/arity misuse is
   usage (exit 2), and nothing panics.
3. Edit a note, re-search: the change is reflected immediately.
4. Related mode: the context pull-out matrix (project title, project
   tag, area title, area tag each surface member entities); context-only
   hits rank below direct hits; renaming a container or tag changes
   related results on the very next search.
5. JSON hits are kind-discriminated complete entity rows; human rows
   show kind, id, title, status, and context; empty searches print
   nothing and `[]`.

## Deferred and parked

Recorded here so milestone wrap-up carries them into the next milestone
file with their revisit triggers:

- **Carried from Milestone 7** — config report generalization,
  genericizing the parallel tag flows, and the typed transition spec
  stay recorded in `plans/MILESTONE_8.md` with their revisit triggers;
  none fire during Search.
- **In-expression scoping operators** (`in:`, `is:`, `~stem`/trigram
  markers) — parked. Revisit when unfiltered search proves too broad in
  daily use; the spellings are reserved by FTS5 rejection today, and the
  virtual index makes alternate tokenizers a per-invocation swap.
- **Embeddings / semantic search** — parked, post-v1. Revisit if
  tag-based topical search (`--related`) proves insufficient in daily
  use; the realistic path is an optional local-encoder sidecar fused
  with FTS, and nothing in this milestone forecloses it.
- **bm25 weight tuning** — the 4/3/2/1 values are a starting point.
  Revisit after real-data use; tests pin ordering properties only, so a
  retune is a one-line change.
