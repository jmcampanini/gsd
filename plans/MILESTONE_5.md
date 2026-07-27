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
- `user_version` bump — the last one; this becomes the Milestone 7
  baseline.

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
- Case-insensitive uniqueness: `Errands` vs `errands` is a `conflict`;
  attach/detach/filter match case-insensitively.
- Deleting a tag detaches it everywhere (join-row CASCADE), reported by
  count. Entity JSON now includes `tags` as an array of names — the
  output contract's final field.
- **Proposed defaults**: duplicate tagging is a no-op success (idempotent
  attach); `tags list` counts usage across all three entity kinds and
  displays alphabetically (per `SCHEMA.md`, tags carry no position).

## Chunks

1. **Tag administration** — `tags` table + joins, `tags add/list/
   rename/delete`, uniqueness and rename semantics.
2. **Attachment everywhere** — `tag`/`untag` verbs on all three nouns,
   `--tag` on the three `add` commands and `list` filter, view + JSON
   enrichment.

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

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] **Schema convergence audit**: dump the dev database schema and diff
      against `SCHEMA.md`'s DDL (tables, checks, indexes, views). Any
      difference is either fixed or logged in `DIVERGENCES.md` and
      consolidated before Milestone 6. This diff is the proof the staged
      construction arrived at the spec.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003.
