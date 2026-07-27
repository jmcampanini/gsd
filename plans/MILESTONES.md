# Milestone Plan (v1)

How gsd gets built: vertical slices. Every milestone ends with working,
demonstrable capability — never a horizontal layer. The spec being sliced
lives in `OVERVIEW.md`, `COMMANDS.md`, and `SCHEMA.md`; each milestone has
its own `MILESTONE_N.md` file with scope, chunks, user stories, and its
agent-verified end-to-end workflow.

## Map

| # | Milestone | Capability delivered | Data mode |
|---|-----------|----------------------|-----------|
| 0 | [Scaffolding](MILESTONE_0.md) | Standards-compliant repo; `gsd --version` | — |
| 1 | [Capture loop](MILESTONE_1.md) | Full task lifecycle in an inbox | throwaway |
| 2 | [Time](MILESTONE_2.md) | Defer/due dates; `available` | throwaway |
| 3 | [Projects](MILESTONE_3.md) | Projects, cascade, `logbook` | throwaway |
| 4 | [Areas](MILESTONE_4.md) | Areas, archive, governing-area inheritance | throwaway |
| 5 | [Tags](MILESTONE_5.md) | Tags across all entities | throwaway |
| 6 | [Config](MILESTONE_6.md) | Config file via go-config-loader; color modes | throwaway |
| 7 | [Go live](MILESTONE_7.md) | Migrations, install story, real-data import | **live** |
| 8 | [Reorder](MILESTONE_8.md) | Manual ordering | live |
| 9 | [Search](MILESTONE_9.md) | FTS5 `search` | live |
| 10 | [Query](MILESTONE_10.md) | Read-only SQL; schema as public contract | live |

The TUI is deliberately **not** in this map — it is a separate effort after
v1 (recorded in `DIVERGENCES.md`).

## Standing process (applies to every milestone)

1. **Plan gate.** The milestone file is reviewed and updated before any code
   is written for it. The file is the contract for the milestone.
2. **Chunks are PRs.** Each milestone is broken into chunks sized for human
   review; one chunk = one PR = one squash merge. Javier reviews every PR —
   that review is the steering mechanism for the codebase.
3. **Guardrails compound.** Anything called out in review that should hold
   forever (style, architecture, safety) gets codified before the next
   milestone — into `AGENTS.md`, lint config, or tests — not left as memory.
4. **User stories are the acceptance test.** Each milestone lists stories in
   the form "you can now X — run this, expect that." The final gate is
   Javier running them by hand.
5. **Agent-verified end-to-end workflow.** Every milestone ends with an
   agent driving the real built binary through its workflow and reporting
   the transcript. Automated e2e tests covering the same ground live in
   `e2e/` and run inside `make check`.
6. **Divergence protocol.** Any departure from `OVERVIEW.md` / `COMMANDS.md`
   / `SCHEMA.md` discovered while building is recorded in `DIVERGENCES.md`
   as it happens. Each entry carries a Consolidate-by deadline; before a
   milestone starts, every entry due by then is consolidated — the spec
   docs are updated to match reality and the entry moves to the closed
   section. A milestone's own "Proposed defaults" (spec-silent behavior
   it defines) are folded into the spec docs at that milestone's exit;
   they need a `DIVERGENCES.md` entry only if they contradict the spec
   rather than extend it.

### Standard exit criteria

Every milestone exits only when all of these hold (plus its own specifics):

- [ ] `make check` green locally and in CI.
- [ ] e2e tests for the milestone's workflow pass inside `make check`.
- [ ] Agent-verified workflow run against the built binary; transcript clean.
- [ ] User stories demoed by Javier.
- [ ] `DIVERGENCES.md` entries due by now consolidated; spec docs match
      shipped behavior, including this milestone's proposed defaults.
- [ ] Review-derived guardrails codified (AGENTS.md / lint / tests).

## Data policy

- **Milestones 1–6 (throwaway):** no migrations. Each schema-changing
  milestone bumps `PRAGMA user_version`, stamped in a dev-only range
  (`9000 + N`) so throwaway stamps can never collide with real migration
  numbers later; on mismatch the binary fails loud with "throwaway db
  from an older milestone — delete it." Dev databases are disposable by
  declaration.
- **Milestone 7 onward (live):** the accumulated schema becomes migration
  `0001_baseline`; every later schema change ships as a numbered migration.
  Real data must survive every milestone from 7 on, per the schema
  stability contract in `SCHEMA.md`.

## Decisions already made (interview, 2026-07-26)

- Stack: Go (current release), cobra, charm.land v2 packages, per
  jmcampanini/cli-standards. Fleet patterns from cmdk/overlay.
- SQLite driver: `modernc.org/sqlite` (pure Go, no cgo; FTS5 supported).
- `--json` + structured errors are a day-one property of every command.
- Schema is created incrementally per milestone (not all up front).
- Real data enters only at Milestone 7, via an agent driving the CLI
  against an export from the current tool — no import code in gsd.
- v1 config surface is minimal: `db_path` and `color` only.
