# Milestone 7 — Go live

Data mode: **live from here on**. Depends on: Milestone 6.

## Capability

gsd stops being a project and becomes the system of record: schema
migrations make the database durable, the install story makes the binary a
resident of the machine, and an agent-driven import moves real data in.
Exit condition is not a feature — it's "Javier lives in gsd."

## Scope

### Migration runner

- Numbered, embedded SQL migrations; `PRAGMA user_version` tracks the
  applied revision; applied automatically on open, each migration in its
  own transaction.
- `0001_baseline.sql` = the accumulated Milestone 1–5 schema, verbatim
  from the Milestone 5 convergence audit (i.e., `SCHEMA.md`).
- A database newer than the binary (`user_version` > known max, below
  the dev-only range) is a fail-loud error ("gsd is older than this
  database").
- Pre-baseline throwaway dbs are refused by name: their `user_version`
  sits in the dev-only range (`9000 + N`, Milestone 1), which can never
  collide with migration numbers, and the runner answers it with the
  delete-your-dev-db message. They hold throwaway data by declaration.
- From here, every schema change ships as a new migration file, and
  `SCHEMA.md`'s stability contract is in force: columns/tables are
  add-only, views only gain columns.

### Install story

- Homebrew HEAD-only formula (`Formula/gsd.rb`, cmdk pattern):
  `head` build with the repo's Go flags, correct version stamping,
  noninteractive functional test (CLI-RELEASE-003).
- README rewritten as landing page: purpose, `--HEAD` install/upgrade,
  representative commands, config discovery (CLI-DOCS-001/002).
- Shell completion generation, wired and documented — gsd now has the
  interactive command surface that warrants it (CLI-CMD-005).

### Import (no gsd code)

- Export data from the current tool (format: whatever it exports —
  decided at import time).
- A Claude session drives the installed `gsd` binary: `tags add` →
  `areas add` → `projects add` → `add`, wiring IDs from `--json` echoes,
  setting defer/due dates and tags as they were.
- The agent then verifies: entity counts by kind match the export, spot
  checks on titles/notes/dates/tags, `available`/`inbox` look right.
- Transcript and count summary saved to `.sandbox/import/` and linked
  from the PR that closes the milestone.

## Chunks

1. **Migration runner** — runner + baseline + newer-db guard + tests
   (fresh db, sequential apply, mid-migration failure rolls back, future
   version refused).
2. **Install story** — formula, README landing page, shell completion,
   `brew install --HEAD` verified on this machine.
3. **Import session** — no code: the agent-driven import itself,
   run against the real config-file-pinned database.

## User stories

### gsd is installed like software, not run like a script

```text
$ brew install jmcampanini/gsd/gsd --HEAD
$ gsd --version
gsd version v0.0.0-89-gdeadbee
```

### Your actual life is in it

```text
$ gsd areas list
  1  Home
  2  Work
$ gsd available
  ... the real things you could actually do today ...
```

### Your data survives upgrades by design

```text
$ brew upgrade --fetch-HEAD gsd    # after any future milestone
$ gsd inbox                        # migrations applied silently; data intact
```

## Agent-verified end-to-end workflow

1. Migration tests inside `make check` (fresh apply, idempotent reopen,
   failure rollback, future-version refusal).
2. `brew install --HEAD` from the tap; `gsd --version` reports commit
   identity; formula's functional test passes.
3. The import session itself — the largest agent-verified workflow yet:
   every entity kind created through the public CLI, counts verified
   against the source export.
4. Post-import: `gsd available`, `inbox`, `logbook` each return sensible
   real data (reviewed by Javier, not asserted by the agent).

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] All open `DIVERGENCES.md` entries consolidated — at go-live the
      spec docs describe the shipped system exactly.
- [ ] Old tool demoted: no longer the capture target (kept read-only or
      retired — Javier's call, recorded here when made).
- [ ] One week of real use with no data-integrity incident before
      Milestone 8 starts.

## Standards

CLI-CMD-005, CLI-RELEASE-001/002/003, CLI-DOCS-001/002/003,
CLI-QUALITY-001.
