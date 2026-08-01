# Milestone 3 — Projects

Data mode: throwaway. Depends on: the consolidated Time baseline.

## Capability

Multi-step work gets a home: projects contain tasks, completing or
cancelling a project cascades to its open tasks with narration, and the
`logbook` interleaves finished tasks and projects. First multi-entity
semantics: containment, re-parenting, RESTRICT deletes.

## Schema delta

- `projects` table per `SCHEMA.md`, minus `area_id` (arrives Milestone 4).
- `tasks` gains `project_id REFERENCES projects(id) ON DELETE RESTRICT`.
  (The containment CHECK needs both container columns, so it arrives with
  `area_id` in Milestone 4.)
- Task insertion's `MAX(position)` append becomes scoped to the destination
  container per `SCHEMA.md` — the capture-loop baseline's whole-table MAX
  would still *look* correct (globally largest = largest in-container), so tests won't
  catch a missed scoping; the predicate must match containment here and
  extend with `area_id` in Milestone 4.
- `inbox` view gains `project_id IS NULL`; `available` gains the
  project-open clause and `project_title` enrichment.
- `logbook` view + command: `kind` discriminator, common column set,
  `resolved_at = COALESCE(done_at, cancelled_at)`, newest first (ordering
  applied by the CLI, not the view). Area/tags columns join in
  Milestones 4–5.
- `idx_tasks_project` created here; `idx_projects_area` waits for
  `area_id` (Milestone 4); `user_version` bump.

## Commands

```text
gsd projects add "TITLE" [--note TEXT|-]
gsd projects list [--status open|done|cancelled|all]
gsd project show N
gsd project edit N [--title TEXT] [--note TEXT|-]
gsd project done N          # cancels remaining open tasks; narrates
gsd project cancel N        # same cascade
gsd project reopen N
gsd project delete N [--recursive]
gsd add "TITLE" [--project N] ...
gsd list [--project N] ...
gsd edit N [--project N | --no-project] ...
gsd logbook
```

## Semantics being proven here

- Cascade narrates (COMMANDS.md § Semantics): project `done`/`cancel`
  cancels open tasks in one transaction and reports each;
  JSON: `{"project": {...}, "cancelled_tasks": [...]}`.
- Re-parenting appends to the destination container's end.
- `delete` honors RESTRICT: non-empty project is an error;
  `--recursive` deletes children in one transaction and narrates.
- Mixed-kind output (logbook) prints the kind next to the ID.
- Completing a task inside a done/cancelled project: blocked —
  **proposed default**: `conflict` (reopen the project first).

## Chunks

1. **Projects exist** — table, `projects add/list`, `project show/edit`,
   `--project` on task `add`/`list`/`edit` with append-on-reparent,
   `inbox`/`available` updates.
2. **Lifecycle + cascade** — `project done/cancel/reopen`, transactional
   cascade with narration, `delete` RESTRICT + `--recursive`.
3. **Logbook** — view + command, interleaving, newest-first, kind
   markers.

## User stories

### You can plan real multi-step work

```text
$ gsd projects add "Kitchen reno"
Added project 1: Kitchen reno
$ gsd add "Get quotes" --project 1
$ gsd add "Pick tiles" --project 1
$ gsd list --project 1
  2  Get quotes
  3  Pick tiles
```

### Finishing a project cleans up after itself, out loud

```text
$ gsd done 2
$ gsd project done 1
Done: project 1  Kitchen reno
Cancelled 1 open task:
  3  Pick tiles
```

### The logbook remembers both kinds

```text
$ gsd logbook
  project  1  Kitchen reno   done       2026-07-26
  task     3  Pick tiles     cancelled  2026-07-26
  task     2  Get quotes     done       2026-07-26
```

## Agent-verified end-to-end workflow

Fresh temp db, `--json`:

1. Create a project and three tasks in it; one loose task in the inbox.
2. `inbox` shows only the loose task; `list --project N` shows three.
3. `done` one project task. `project done N`: response contains the
   project row plus exactly the two remaining open tasks as
   `cancelled_tasks`.
4. `logbook` returns four entries (1 project, 3 tasks), newest first,
   each with correct `kind`.
5. `project reopen N`; verify cancelled tasks stay cancelled (reopen is
   not un-cascade). Re-complete.
6. `projects add` + one task; `project delete` → error (RESTRICT);
   `project delete --recursive` succeeds and narrates the child.
7. Re-parent a task between two projects; verify it appended last.
8. Grammar guard: bare `gsd projects` (no verb) is a usage error, exit 2
   (COMMANDS.md § Grammar; areas/tags nouns follow the same pattern in
   their milestones).

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `OVERVIEW.md` § "Completion cascade and deletion" verified against
      shipped behavior word by word (first place spec drift is likely).

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003.
