# Milestone 7 — Reorder

Data mode: throwaway. Depends on: Milestone 6.

Written lighter than the earlier milestones on purpose: later consolidations may reorder 7–9 or reshape their scope. Re-review this file at its plan gate.

## Capability

Manual ordering — the thing a list tool is for. `position` has existed since the capture-loop baseline (append-only); this milestone makes it manipulable. It needs **no schema change**.

## Commands

```text
gsd reorder N          (--after M | --before M | --first | --last)
gsd project reorder N  (--after M | --before M | --first | --last)
gsd area reorder N     (--after M | --before M | --first | --last)
```

## Semantics (per COMMANDS.md)

- Sibling-relative only: the reference entity must live in the same container (same project/area/inbox for tasks; same area or standalone group for projects; the global list for areas). Cross-container reference is `invalid_argument`.
- Reorder renumbers the container (cheap at this scale, per `SCHEMA.md`).
- JSON echo: **proposed** — the reordered entity plus its container's new ordering (`{"task": {...}, "container": [ids in order]}`), so agents see the result without a second call.

## Chunks

1. **The whole verb** — grammar, all three nouns, renumbering, errors, e2e. Single chunk; split only if review wants it.

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

Fresh temp db, seeded across all three container kinds:

1. Reorder matrix per noun: `--first`, `--last`, `--after`, `--before`; assert full container ordering after each.
2. Cross-container and self-reference errors.
3. Mixed-operation stability: add, done, reorder, delete interleaved; ordering stays consistent, no position collisions.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)).

## Standards

CLI-CMD-002/003.
