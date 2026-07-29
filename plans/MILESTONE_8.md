# Milestone 8 — Reorder

Data mode: live. Depends on: Milestone 7.

Written lighter than the pre-go-live milestones on purpose: the Milestone 7
consolidation and a week of real use may reorder 8–10 or reshape their
scope. Re-review this file at its plan gate.

## Capability

Manual ordering — the thing a list tool is for. `position` has existed
since the capture-loop baseline (append-only); this milestone makes it
manipulable. Chosen as the first live-data milestone because it needs **no schema
change**: zero migration risk while the migration muscle is fresh.

## Commands

```text
gsd reorder N          (--after M | --before M | --first | --last)
gsd project reorder N  (--after M | --before M | --first | --last)
gsd area reorder N     (--after M | --before M | --first | --last)
```

## Semantics (per COMMANDS.md)

- Sibling-relative only: the reference entity must live in the same
  container (same project/area/inbox for tasks; same area or standalone
  group for projects; the global list for areas). Cross-container
  reference is `invalid_argument`.
- Reorder renumbers the container (cheap at this scale, per `SCHEMA.md`).
- JSON echo: **proposed** — the reordered entity plus its container's new
  ordering (`{"task": {...}, "container": [ids in order]}`), so agents
  see the result without a second call.

## Chunks

1. **The whole verb** — grammar, all three nouns, renumbering, errors,
   e2e. Single chunk; split only if review wants it.

## User stories

### The important thing sits on top because you put it there

```text
$ gsd inbox
  4  Call plumber
  9  Renew passport
$ gsd reorder 9 --first
$ gsd inbox
  9  Renew passport
  4  Call plumber
```

### Order is honest about its scope

```text
$ gsd reorder 9 --after 12     # 12 lives in some project
{"error": {"code": "invalid_argument", "message": "task 12 is in a different container"}}
```

## Agent-verified end-to-end workflow

On a **copy** of the live database (never the live file):

1. Reorder matrix per noun: `--first`, `--last`, `--after`, `--before`;
   assert full container ordering after each.
2. Cross-container and self-reference errors.
3. Mixed-operation stability: add, done, reorder, delete interleaved;
   ordering stays consistent, no position collisions.
4. On the real db, one reorder + `gsd inbox` visual check by Javier.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)).

## Standards

CLI-CMD-002/003.
