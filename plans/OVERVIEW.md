# gsd — Distilled Design Spec (v1)

`gsd` (get shit done) is a CLI (with a post-v1 TUI planned) wrapping a
personal to-do system. Its design goals are simple, extendable primitives, a
Things-inspired workflow, and a SQLite backend. The current baseline provides
the repository and CLI foundation, a complete bare-inbox task lifecycle,
calendar-aware due dates, deferrals, and an `available` view, projects
with task containment, narrated completion cascades, and the interleaved
`logbook`, areas holding projects and loose tasks with governing-area
archiving and RESTRICT-guarded recursive deletion, and flat tags spanning
all three entity kinds. Tags complete the v1 entity model and its JSON field
set; the broader command surface remains the forward-looking canonical v1
target delivered incrementally through `MILESTONES.md`. The SQL schema lives
in `SCHEMA.md`.

## Primitives

**Area** — a container for projects and loose tasks. Never completes.

- `title`, `note`
- `position`
- `archived_at` (retire an area without deleting it; empty = active)

**Project** — belongs to at most one area. Carries no dates.

- `title`, `note`
- `done_at` / `cancelled_at` — at most one set; both empty = open
- `position`

**Task** — belongs to exactly one of: a project, an area (loose task), or
nothing (= inbox).

- `title`, `note`
- `defer_until` (optional) — calendar day; hides the task until it arrives
- `due_on` (optional) — calendar day; reserved for real external deadlines
  only, never aspirations
- `done_at` / `cancelled_at` — at most one set; both empty = open
- `position`

**Tag** — flat namespace, case-insensitively unique with SQLite's ASCII-only
`NOCASE`. Attaches to tasks, projects, and areas; names are its identity.

## Field conventions

- `*_at` — an instant (UTC timestamp), stamped by the system.
- `*_on` / `*_until` — a calendar day (no timezone), chosen by the human.
- No boolean fields: a flag is an event in disguise, stored as a nullable
  `*_at` (empty = hasn't happened).

## Ordering

`position` is an integer giving manual sort order within a container — the
order you see when you drag things around: projects within an area, tasks
within a project. Without it, every list
falls back to alphabetical or created-at, which never matches how you
actually think about the work.

## State machine

`open → done` or `open → cancelled`. State is the timestamp pair itself:
completing stamps `done_at`, cancelling stamps `cancelled_at`, and the two
are mutually exclusive. Reopening clears the stamp. `status` is derived
from the pair, never stored.

Hard delete exists but is the uncommon path — the normal end of life is
`done` or `cancelled`, which lands in the logbook.

## Views

- **Inbox**: open tasks with no project and no area.
- **Available**: task is open, its project (if any) is open, its area
  (own, or inherited through its project) is not archived, and
  `defer_until` is empty or ≤ today.
- **Logbook**: everything done or cancelled — tasks and projects — ordered
  by resolution time, newest first; a project lists above the tasks its
  cascade cancelled at the same instant.

## Tag lifecycle

- Tags are created explicitly and addressed by name. Valid nonblank UTF-8
  spelling is stored unchanged; lookups are case-insensitive for ASCII and
  display the stored spelling, initially the spelling used at creation.
- Attach and detach are idempotent content edits on tasks, projects, and
  areas. Multi-name changes collapse duplicates and are all-or-nothing;
  tagged entity creation is transactional. Tagging remains allowed under
  archived areas and resolved projects and does not bump entity `updated_at`.
- Renaming updates the one shared tag, so every entity reflects the new name.
  Deleting a tag detaches it everywhere. Deleting a tagged entity removes its
  attachments, reducing usage counts across the three entity kinds.
- Every task, project, and area JSON row carries a non-null `tags` array.
  Tag listings add a cross-entity `usage_count`; tag deletion reports its
  `detached` count.
- Human tag lists show name and usage count without IDs. Entity `show` output
  has a `Tags` row, collections add no tags column, and mutations render the
  stored spelling.

## Completion cascade and deletion

- Completing a project auto-cancels its remaining open tasks (CLI reports
  what it cancelled).
- Cancelling a project cancels its open tasks.
- Reopening a project clears only its exit: the cascade is never undone,
  and tasks it cancelled stay cancelled until individually reopened.
- A done or cancelled project is closed history: completing, cancelling, or
  reopening its tasks, adding tasks into it, and moving tasks in or out are
  conflicts until the project is reopened. Content edits and hard deletes
  of contained tasks stay allowed.
- An archived area is retired history through the governing area (own, or
  inherited through the project): creating into it, moving projects or
  tasks in or out of it, and completing, cancelling, or reopening anything
  it governs are conflicts until the area is unarchived. Content edits and
  hard deletes stay allowed; archiving never mutates contents, and
  unarchive restores visibility with every position intact.
- Hard deletes never destroy other entities: deleting a non-empty area or
  project is an error — archive it (areas) or empty it first. A recursive
  delete exists only as an explicit CLI opt-in (`--recursive`), never as a
  side effect.
- Tag attachments are parts, not entities: they go with their owner.
  Deleting a tag detaches it from everything it was on.

## Deferred to v2+

- **Recurrence** — decided but postponed: both trigger types (`schedule`:
  calendar-driven; `completion`: interval after last done), implemented as
  clone-based spawning, never two open instances of one rule.
- **Sequential/parallel project modes** — dropped for now. If re-added:
  a `mode` column on project plus one extra clause in the Available filter.
- **Today** — no Today concept in v1. When added, it will be stored/manual
  (a marker you set), not derived from dates: a `today_at` stamp, plus a
  `today_position` column if an ordered Today list is wanted.
- **Tag hierarchy** — start flat.
- **Sub-subtasks** — never. A task is atomic; if it has real sub-structure,
  it's a project.

## Defaults chosen along the way (flag if wrong)

- Long-form field is `note` on all three entities.
