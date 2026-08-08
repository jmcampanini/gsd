# Temporary Divergence Intake

`OVERVIEW.md`, `COMMANDS.md`, and `SCHEMA.md` are the canonical product
specifications. This file is only an intake queue for a temporary mismatch
discovered during implementation; it must not become a parallel permanent
specification or decision archive.

## Protocol

1. Record an actual contradiction with a canonical specification as soon as it
   is discovered. Divergence numbers are assigned monotonically and never
   reused. Include the affected section, the temporary behavior or proposed
   decision, an owner, and a consolidation deadline.
2. Before that deadline, reconcile the decision into the active milestone plan
   and the relevant canonical specification, align code and tests, and verify
   the resulting behavior.
3. Delete the entry during milestone consolidation. Do not retain a closed
   section; git history preserves the temporary discussion.

Spec-silent behavior that merely extends the target should be added to the
active plan and folded into the canonical specification during consolidation
without creating a divergence entry.

Entry format:

```text
## D-NNN: short title
- Recorded: date and planning/implementation context
- Diverges from: canonical document and section
- Temporary behavior or decision: what differs and why
- Owner: person responsible for reconciliation
- Consolidate by: milestone boundary
- Reconciliation: milestone and canonical sections that must change
```

## Intake

## D-004: TUI section predates the Navigator plan gate
- Recorded: 2026-08-08, Milestone 12 plan gate
- Diverges from: `COMMANDS.md` § TUI (post-v1) — the root-tree shape
  ("loose projects, then areas with their open projects nested") and
  "`/` is incremental search with the same semantics as `gsd search`"
- Temporary behavior or decision: Navigator ships the two-lens tree
  (root → Boards/Areas → projects → tasks, with a `(no area)`
  pseudo-row) and `/` as an in-memory fuzzy filter (snacks-picker
  semantics) over the current view; `gsd search` keeps FTS semantics
  in the CLI
- Owner: Javier
- Consolidate by: Milestone 12 consolidation
- Reconciliation: `COMMANDS.md` TUI section (tree shape, `/`
  semantics) and the shipped `tui` command documentation, which joins
  the `--json`-rejecting exceptions

_The next entry number is D-005._
