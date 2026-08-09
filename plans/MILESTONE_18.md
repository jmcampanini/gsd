# Milestone 18 — History

Data mode: **live**. Depends on: Milestone 9. **Optional**:
deliberately underspecified; enters planning only when its trigger
fires. Sequenced last.

Written as a placeholder; the design interview happens at activation.

## Capability

Track how the system changes over time: today gsd stores current
state plus resolution timestamps, and the logbook answers "what
finished" — but not "how did this get here" (stage transitions and
their timing, edits, reorderings, per-stage aging on boards). This
milestone gives change-over-time a durable home.

Recording begins when this milestone lands, not retroactively — the
boards interview (2026-08-07) accepted that consequence when it chose
current-stage-only storage for Milestone 11.

## Scope

Deliberately unwritten. Candidate questions for the activation
interview: which changes are worth recording (stage movement only, or
all mutations), event log versus periodic snapshot, retention, and
what surfaces read it (`logbook` extensions, `board show` aging, a
`history` verb). Nothing is settled here.

## Activation trigger

Daily use asks a question about the past that current state and the
logbook cannot answer — the recorded example: "how long do projects
sit in each stage?"

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)); real criteria
are written at activation.
