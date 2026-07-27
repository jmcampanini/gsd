# Repository guidance

## Build and verification

- Use Make targets instead of raw `go` or tool commands.
- Run `make help` to discover available targets.
- Run `make check` for the complete local verification contract.

## Scratch files

- Keep repository-local scratch work under `.sandbox/` at the repository root.

## Code comments

- Add a comment only to explain why a non-obvious choice is necessary.

## Review guardrails

Add durable, repository-specific guardrails here when review identifies a recurring risk or convention.

<!-- cli-standards: CLI-AGENTS-001 -->
