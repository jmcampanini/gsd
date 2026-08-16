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

_The next entry number is D-005._
