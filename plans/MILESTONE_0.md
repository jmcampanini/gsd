# Milestone 0 — Scaffolding

Data mode: none (no database yet). Depends on: nothing.

## Capability

A standards-compliant, CI-protected repository producing a real binary.
`gsd --version` reports git-derived identity; `make check` is the single
verification contract, locally and in CI. No domain behavior yet — this
milestone exists so every later chunk lands on rails.

## Scope

- Go module `github.com/jmcampanini/gsd`, current Go release.
- Root `main.go` owning process exit; commands in `cmd/` (CLI-LAYOUT-001/002).
- Root command: `SilenceUsage`/`SilenceErrors`, version via ldflags,
  fresh command tree per execution (CLI-CMD-001).
- Exit-code contract wired at the root: `0` success, `1` domain error,
  `2` usage error (cobra parse/unknown-command failures mapped to 2).
- Fleet Makefile: `build` (to `build/gsd`, git-describe version injection),
  `test` (race), `lint`, `vuln` (govulncheck), `fmt`/`fmt-check`,
  `tidy`/`tidy-check`, `check` (includes `vuln` — no silently omitted
  capability), `clean`, `help` (CLI-QUALITY-001/002/004,
  CLI-RELEASE-002).
- golangci-lint + govulncheck as versioned tools in `go.mod`.
- `e2e/` package skeleton: builds the binary, runs it as a subprocess
  (harness only; first real workflows arrive in Milestone 1).
- Minimal `README.md`: purpose, build, verify — rewritten as the full
  landing page in Milestone 7 (CLI-DOCS-001).
- CI workflow running `make check`; dependabot (gomod + actions);
  scheduled update-go workflow (CLI-GO-002); Dependency Review workflow
  and an observe-first CodeQL run (CLI-SEC-004).
- `AGENTS.md` filled in: make-targets-only rule, `.sandbox/` scratch
  convention, comment policy, review-guardrail section that grows over
  time. `CLAUDE.md` stays exactly `@AGENTS.md` (CLI-AGENTS-001/002).
- `SECURITY.md`; live GitHub settings per CLI-SEC-001..006 and
  CLI-PR-001/002: main ruleset requiring `check`, Actions read-only +
  SHA pinning, squash merge from PR title/body, secret scanning + push
  protection, private vulnerability reporting, `allow_update_branch`.

Out of scope: any command beyond help/version, SQLite, Homebrew formula
(arrives with the install story in Milestone 7).

## Chunks

1. **Buildable skeleton** — module, `main.go`, `cmd/root.go`, exit-code
   mapping, Makefile, `.golangci.yml`, `.gitignore`, e2e harness skeleton.
   Reviewable as: "is this the shape every future PR builds on?"
2. **Automation and settings** — CI, dependabot, update-go, Dependency
   Review, CodeQL (observe first), `SECURITY.md`, `README.md`,
   `AGENTS.md` content, live GitHub settings (applied via `gh`, with the
   checklist and verification values recorded in the PR).

## User stories

### You can build and verify the project with one command each

```text
$ make build
$ ./build/gsd --version
gsd version v0.0.0-1-gabc1234
$ make check
... fmt-check tidy-check lint test all green
```

### The CLI already behaves like a fleet citizen

```text
$ ./build/gsd nonsense
Error: unknown command "nonsense" for "gsd"
$ echo $status
2
```

## Agent-verified end-to-end workflow

1. `make check` — exits 0.
2. `make build`; `./build/gsd --version` — output contains the current
   `git describe` value.
3. `./build/gsd --help` — exits 0, usage on stdout.
4. `./build/gsd nonsense` — exits 2, diagnostic on stderr, stdout empty.
5. CI run on the PR is green with `check` as the required context.

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] Live GitHub settings verified: `allow_update_branch: true`,
      `allow_squash_merge: true`, squash title/body from PR, main ruleset
      requires `check`.
- [ ] `AGENTS.md` reviewed by Javier — it is the seed of the guardrail
      system, so its initial content is itself a review artifact.

## Standards

CLI-LAYOUT-001/002/003, CLI-CMD-001/002/003, CLI-QUALITY-001/002/004,
CLI-GO-001/002/003, CLI-SEC-001..006, CLI-PR-001/002, CLI-RELEASE-002,
CLI-AGENTS-001/002, CLI-DOCS-001.
