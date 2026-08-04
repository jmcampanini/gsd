## Build and verification

- Use Make targets instead of raw `go` or tool commands.
- Run `make help` to discover available targets.
- Run `make check` for the complete local verification contract.

## Command architecture

- Cobra commands only adapt arguments, flags, streams, and presentation. Put use-case validation and orchestration in the relevant `internal` service.
- Services depend on narrow store interfaces and are the sole owners of semantic validation for use-case inputs. Stores assume those inputs were validated; store checks are limited to caller-contract failures needed to construct a valid operation and are not a second user-facing validation layer. Services own transaction boundaries that coordinate multiple store operations; stores own atomicity for a single operation when its implementation needs multiple statements. Use one atomic SQL statement instead of a transaction when it proves the complete operation.
- Concrete SQLite code, schema bootstrap, and row scanning belong in `internal/store`; do not expose `database/sql` types across the store boundary.
- Open runtime dependencies inside behavioral command execution through an injected factory. Help, version, and argument parsing must not open the database.
- Return stable coded application errors from service/store boundaries. Convert unexpected errors to `internal` and Cobra syntax failures to `usage` at the root adapter. In cmd, any uncoded error maps to `usage` (exit 2) because Cobra parse failures arrive uncoded: wrap application errors in `normalizeApplicationError` and build deliberate usage errors with `usageError`.
- Service and store error messages state semantics only. Recovery guidance that names CLI flags or command spellings is presentation and is composed by the command adapter.
- Route all success and error rendering through shared writers so JSON remains compact and newline-terminated, errors stay on stderr, and human output stays on stdout.
- Task, project, and area tag service flows are intentionally parallel. When changing one, compare it with both siblings and keep only entity-specific differences.
- Parameterize duplicated flows only when the variation is data or a single function value. Keep type-varying orchestration flows concretely parallel until divergence proves the coordination cost.

## Test ownership

- Use real temporary SQLite databases for schema, bootstrap, pragma, transaction, and query semantics.
- Test validation and orchestration at the service layer with store-boundary fakes.
- Test streams, JSON envelopes, and exit mapping at the command layer. Reserve subprocess tests for persistence and complete binary wiring across invocations.
