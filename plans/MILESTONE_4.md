# Milestone 4 — Areas

Data mode: throwaway. Depends on: Milestone 3.

## Capability

Life gets its containers: areas hold projects and loose tasks, archiving
an area retires a whole slice of life without deleting anything, and
`available` respects the governing area — own, or inherited through the
project. The entity graph is now structurally complete.

## Schema delta

- `areas` table per `SCHEMA.md`.
- `projects` gains `area_id` (RESTRICT); `tasks` gains `area_id`
  (RESTRICT) plus the containment
  `CHECK (project_id IS NULL OR area_id IS NULL)` — schema-enforced
  mutual exclusion at last.
- Indexes: `idx_tasks_project`, `idx_tasks_area`, `idx_projects_area`.
- Views rebuilt to their near-final form: `inbox` (both containers NULL),
  `available` (archived-governing-area exclusion via
  `COALESCE(t.area_id, p.area_id)`), `logbook` (governing area columns).
  Only the `tags` enrichment column remains (Milestone 5).
- `user_version` bump.

## Commands

```text
gsd areas add "TITLE" [--note TEXT|-]
gsd areas list [--archived]
gsd area show N
gsd area edit N [--title TEXT] [--note TEXT|-]
gsd area archive N
gsd area unarchive N
gsd area delete N [--recursive]
gsd projects add "TITLE" [--area N] ...
gsd projects list [--area N] ...
gsd project edit N [--area N | --no-area]
gsd add "TITLE" [--area N] ...          # loose task
gsd list [--area N] ...
gsd edit N [--area N | --no-area] ...
```

## Semantics being proven here

- `--project` and `--area` together on a task is `invalid_argument`
  (matches the CHECK).
- Archiving hides everything the area governs from `available` — loose
  tasks directly, project tasks through inheritance — while `list` and
  `show` still see them. Unarchive restores; position preserved.
- `area delete` RESTRICT on any contained project or task;
  `--recursive` is the explicit one-transaction opt-in.
- **Proposed default**: `areas list` hides archived areas unless
  `--archived`, which lists only archived ones.

## Chunks

1. **Areas exist** — table, area CRUD, `--area` on tasks and projects,
   containment CHECK + mutual-exclusion errors, view updates for
   governing-area columns.
2. **Archive semantics** — archive/unarchive, `available` exclusion by
   inheritance, `--archived` listing, `delete --recursive`.

## User stories

### Your projects and loose tasks live where they belong

```text
$ gsd areas add "Home"
$ gsd project edit 1 --area 1        # Kitchen reno → Home
$ gsd add "Change furnace filter" --area 1
```

### You can put a whole part of life on ice

```text
$ gsd area archive 1
Archived: area 1  Home
$ gsd available          # furnace filter AND all Kitchen reno tasks gone
$ gsd area unarchive 1   # everything back, order intact
```

### The structure defends itself

```text
$ gsd add "Impossible" --project 1 --area 1
{"error": {"code": "invalid_argument", "message": "--project and --area are mutually exclusive"}}
$ gsd area delete 1
{"error": {"code": "conflict", "message": "area 1 is not empty"}}
```

## Agent-verified end-to-end workflow

Fresh temp db, `--json`:

1. Build: area A with a loose task and a project holding two tasks;
   area B with one loose task; one inbox task.
2. `available` shows all five tasks; each row's `governing_area_id`
   is correct (inherited for project tasks, NULL for inbox).
3. `area archive A`: `available` drops to area-B task + inbox task.
   `list --area A` still sees the loose task.
4. `unarchive`; all five return; `areas list` / `--archived` partition
   correctly during 3–4.
5. Mutual exclusion: task add and edit with both containers → error.
6. Re-parent the project to area B; its tasks' governing area follows
   and the project appends to the end of B's project list.
7. Delete a task inside the project and the loose task in area A — task
   deletes never block (RESTRICT is for containers only).
8. `area delete` on non-empty A → `conflict`; empty B of its task, delete
   B → succeeds.

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] `available` view SQL diffed against `SCHEMA.md`'s definition —
      remaining delta must be exactly the `tags` column.

## Standards

CLI-CMD-002/003, CLI-OUTPUT-003.
