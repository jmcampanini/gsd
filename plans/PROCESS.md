# Development Process

This is the canonical contract for taking a gsd milestone from plan to
`main`. Milestone files are temporary execution plans; `OVERVIEW.md`,
`COMMANDS.md`, and `SCHEMA.md` are the canonical product specifications.

## Branch roles and ancestry

- **`main`** contains only reviewed, consolidated milestones.
- **The milestone branch** starts from the current `main` tip and integrates
  every chunk for one milestone. It is the target of chunk pull requests and,
  once consolidated, the source of the milestone pull request to `main`. It is
  deleted after that pull request is reviewed and squash-merged into `main`.
- **A chunk branch** starts from the current milestone-branch tip. Its pull
  request targets the milestone branch, never `main`. It is deleted after that
  pull request is squash-merged. The next chunk starts from the new
  milestone-branch tip so its ancestry includes all accepted work.

Branch names may follow the repository's current convention; the roles,
ancestry, and pull-request targets above are the contract, not exact name
strings.

## Milestone workflow

1. **Plan the milestone.** Review and update its milestone file before writing
   implementation code. Define a demonstrable capability, reviewable chunks,
   test ownership, user stories, and an agent-verified end-to-end workflow.
   One chunk must be small enough for meaningful human review.
2. **Create the milestone branch.** Branch from the current `main` tip. This is
   the integration line for the milestone.
3. **Deliver each chunk in sequence.** For each planned chunk:
   1. branch from the current milestone-branch tip;
   2. implement the chunk and its cheapest faithful verification;
   3. run `make check` locally;
   4. open a pull request targeting the milestone branch and require green CI;
   5. have Javier code-review it, codify any lasting review guardrail, and
      squash-merge it; then
   6. start the next chunk from the resulting milestone-branch tip.
4. **Review the complete milestone end to end.** After every chunk is merged,
   run the real built binary through the milestone's documented workflow and
   retain a clean transcript. The equivalent durable subprocess coverage
   belongs in `e2e/` and runs inside `make check`. Javier then performs the
   milestone's user-story demo; the agent workflow does not replace that human
   acceptance gate.
5. **Consolidate the milestone.** Reconcile the completed implementation as
   described below, then rerun verification. If consolidation changes
   behavior, repeat the affected end-to-end and user-story checks.
6. **Land the milestone.** Open the consolidated milestone branch as a pull
   request targeting `main`. Review it as a complete vertical slice, require
   green CI, and squash-merge it into `main`.
7. **Begin the next milestone.** Plan and branch it only from the new `main`
   tip, then repeat this workflow.

## Milestone consolidation

Consolidation turns an execution branch into an authoritative product
baseline:

- Reconcile `OVERVIEW.md`, `COMMANDS.md`, and `SCHEMA.md` with all decided
  behavior. Fold in accepted proposed defaults and correct any temporary
  implementation/specification mismatch.
- Resolve every temporary divergence due at this boundary. Update the relevant
  milestone file while it is still useful, update the canonical specification,
  and remove the intake entry from `DIVERGENCES.md`. That file is temporary
  intake, not a parallel specification or a permanent decision archive.
- Reconcile tests with the stable observable contract and verify the complete
  workflow. For completed capabilities, current code, tests, and canonical
  specifications become authoritative together.
- Codify review findings that must hold in future work in `AGENTS.md`, lint or
  build configuration, or tests. Do not leave permanent guardrails only in
  review comments or memory.
- Update the roadmap and all links, then retire the completed milestone plan.
  **Plan deletion is permanent:** completed execution plans are deleted, not
  archived, and must not remain as an alternative description of shipped
  behavior. Git history retains the implementation record.

## Standard exit workflow

A milestone exits only when all of these gates, plus its own specific exit
criteria, hold:

- [ ] Every planned chunk was reviewed, passed local `make check` and CI, and
      was squash-merged into the milestone branch in sequence.
- [ ] The milestone's automated end-to-end workflow passes from `e2e/` inside
      `make check`.
- [ ] An agent drove the real built binary through the documented end-to-end
      workflow and reported a clean transcript.
- [ ] Javier successfully demoed the milestone's user stories.
- [ ] Canonical documentation was reconciled with decided and shipped
      behavior, and temporary divergence entries due now were removed.
- [ ] Review-derived guardrails were codified in the repository.
- [ ] The completed plan was deleted and roadmap/document links were checked.
- [ ] `make check` is green on the consolidated milestone branch and the final
      pull request's CI is green.
- [ ] The consolidated milestone pull request was reviewed and squash-merged
      into `main`.
