# Milestone 16 — Serve

Data mode: **live**. Depends on: Milestone 9. **Optional**: enters
planning only when a consumer that can't exec the CLI appears (browser
frontend, remote agent); sequenced after the TUI (Milestones 10 and
12–15).

Written ahead; re-review at plan gate on activation.

## Capability

`gsd serve` turns the CLI's capabilities into a loopback-only HTTP API: a
second first-class adapter over the same services, speaking the same JSON
the CLI already emits under `--json`. Agents and future frontends get an
ergonomic, native HTTP grammar — resources for nouns, intent verbs for
transitions — with no second validation layer, no second wire format, and
no change to the data contract.

## Scope

### Command

```text
gsd serve [--addr HOST:PORT]
```

- Runs until SIGINT/SIGTERM, then drains in-flight requests and exits `0`.
- Startup announces the bound address on stdout; request and diagnostic
  lines go to stderr. `--json` does not apply to `serve` itself.
- The address is config key #2: TOML `[serve] addr`, env `GSD_SERVE_ADDR`,
  flag `--addr`, standard precedence; default `127.0.0.1:8473`. A
  non-loopback host is `invalid_argument` at startup — loopback-only and
  no auth is the v1 exposure contract. Key #2 trips the deferred
  config-report generalization recorded in `MILESTONE_12.md` (source
  classification pushed into go-config-loader's `configreporter`);
  `gsd config --provenance` reports the new key like `db_path`.

### API grammar

- Everything under `/v1`. Wire bodies are the CLI `--json` shapes with
  their conventions intact: bare entity objects, bare arrays, cascade
  envelopes, snake_case, `null` for empty nullables, `[]` for empty
  collections, one compact JSON value per response.
- Nouns are resources; tasks, projects, and areas are addressed by ID;
  tags by name (URL-encoded, case-insensitive resolution to the stored
  spelling).
- Lifecycle transitions are action endpoints (`POST …/done`), returning
  the CLI's envelopes — a transition report, not just the updated row.
- `PATCH` owns content edits: merge-patch semantics, absent = unchanged,
  explicit `null` clears (the `--no-due` twin); an empty patch is
  `invalid_argument`. Tag rename is `PATCH /v1/tags/{name}`
  (`{"title":…}`).
- Creates return `201` with a `Location` header and the entity echo;
  every other success returns `200` with the payload the CLI prints,
  including deletes (the deleted entity, or the recursive-deletion
  envelope).
- Errors reuse `{"error":{"code","message"}}` on the wire: `not_found`
  404, `invalid_argument` 400, `conflict` 409, `internal` 500; wrong
  method 405; malformed or unknown-field bodies are `invalid_argument`.
  Messages stay semantic — no CLI flag spellings, no HTTP-composed
  recovery guidance in v1.

### Endpoints

| Family | Endpoints |
|---|---|
| Tasks | `POST/GET /v1/tasks` · `GET/PATCH/DELETE /v1/tasks/{id}` · `POST /v1/tasks/{id}/…` for `done`, `cancel`, `reopen`, `tag`, `untag`, `reorder` |
| Projects | same shape as tasks; `done`/`cancel` return the cascade envelope; `DELETE …?recursive=true` |
| Areas | `POST/GET /v1/areas` · `GET/PATCH/DELETE /v1/areas/{id}` · `POST /v1/areas/{id}/…` for `archive`, `unarchive`, `tag`, `untag`, `reorder` · `DELETE …?recursive=true` |
| Tags | `POST/GET /v1/tags` · `PATCH/DELETE /v1/tags/{name}` |
| Views | `GET /v1/inbox` · `GET /v1/available` · `GET /v1/logbook` |
| Search | `GET /v1/search?q=EXPR&…` |

- `tag`/`untag` take `{"names":[…]}` and keep the CLI's multi-name
  all-or-nothing atomicity and entity echo; `reorder` takes exactly one
  of `{"after":N}`, `{"before":N}`, `{"first":true}`, `{"last":true}`.
- List filters are query parameters with the CLI's mutual-exclusion
  rules (**proposed** spellings: `status`, `project`, `area`, `tag`,
  `due=true`/`overdue=true`/`deferred=true`, `archived=true`/`all=true`).
- Collections return everything; no pagination at personal scale.

### Semantics

- Handlers are thin adapters in `internal/httpapi` over the existing five
  `Application` interfaces — services keep all validation and transaction
  boundaries; `serve` adds one long-lived construction path beside the
  per-invocation factory. Stack is stdlib `net/http`; no new
  dependencies.
- The store is untouched: single pinned connection, non-WAL; handlers
  serialize process-wide, and `BEGIN IMMEDIATE` plus the busy timeout
  keep concurrent CLI + HTTP writers safe — races resolve into
  `conflict` responses, never corruption.
- The concurrency contract is documented, not enforced: `PATCH` is
  last-write-wins per field, creates are not idempotent; `updated_at` is
  named as the future `If-Match` validator. Stated plainly in `API.md`.

### Canonical contract

`plans/API.md` lands with this milestone as the canonical HTTP contract,
a peer of `COMMANDS.md`: resources, grammar rules, endpoint table, wire
and error contract, concurrency contract, and the serve config key. No
OpenAPI document in v1. `COMMANDS.md` gains the `serve` command and the
second config key.

### Deliberately deferred, with revisit triggers

- **Schema-revision skew guard** (long-lived server vs. a migrated db) —
  first post-baseline migration; until one ships, skew is impossible.
- **Browser-borne request hardening** (Host allowlist, `Origin`
  rejection, strict `Content-Type`) — before any browser-based consumer.
- **WAL + wider connection pool** — observed contention (busy-driven
  500s).
- **`ETag`/`If-Match` and idempotency keys** — a collaborative or
  retry-heavy consumer.
- **OpenAPI document** — a consumer that needs codegen.
- **`SQLITE_BUSY` → 503 mapping** — if busy 500s appear in practice.

## Chunks

1. **Serve spine and views** — `serve` command; `[serve] addr` with full
   precedence, loopback validation, and the config-report
   generalization; long-lived wiring; `internal/httpapi` router, error
   mapping, logging, graceful shutdown; `GET /v1/inbox`, `/v1/available`,
   `/v1/logbook`.
2. **Tasks over HTTP** — the complete task family: create, list with
   filters, show, edit, transitions, tag/untag, reorder, delete.
3. **Projects and areas over HTTP** — CRUD, transitions with cascade
   envelopes, archive/unarchive, reorder, tag/untag, recursive deletes.
4. **Tags and search** — tag administration by name, `GET /v1/search`
   with filter composition, and the wire-contract equivalence sweep.

Chunk demos show the real HTTP surface: captured `curl` exchanges against
a served temporary database. The wire JSON *is* this surface's native
output; the never-`--json` demo rule governs CLI surfaces, not this one.

## User stories

### The same system, over a socket

```text
$ gsd serve
gsd serve listening on 127.0.0.1:8473
```

```text
$ curl -s localhost:8473/v1/tasks -d '{"title":"Call plumber","tags":["home"]}'
{"id":42,"project_id":null,"area_id":null,"title":"Call plumber",…,"tags":["home"]}
$ curl -s -X POST localhost:8473/v1/tasks/42/done
{"id":42,…,"status":"done",…}
```

### Cascades narrate over the wire

```text
$ curl -s -X POST localhost:8473/v1/projects/7/done
{"project":{"id":7,…,"status":"done"},"cancelled_tasks":[{"id":23,…},{"id":31,…}]}
```

### Loopback is the contract

```text
$ gsd serve --addr 0.0.0.0:8473
Error: serve address must be loopback: 0.0.0.0:8473
```

## Agent-verified end-to-end workflow

Against the real built binary and a temporary database:

1. Boot `gsd serve` on an ephemeral loopback port; the startup line
   appears; SIGINT drains an in-flight request and exits `0`.
2. Address contract: file/env/flag precedence for `serve.addr`;
   `gsd config --provenance` classifies the new key; a non-loopback
   address is refused as `invalid_argument`, exit `1`.
3. Full-surface pass over HTTP: tags → area → project → tasks; every
   endpoint family exercised; `201` + `Location` on creates; entity
   echoes on every mutation.
4. Wire equivalence: for representative operations, HTTP bodies decode
   to the same field sets and values as the CLI's `--json` output for
   the same state — cascade, deletion, and error envelopes included.
5. Error matrix: 404/400/409 mapping, 405 on wrong methods, malformed
   body, empty patch, filter mutual exclusions, unknown tag,
   resolved-project and archived-area conflicts.
6. Concurrency: interleaved CLI and HTTP writers (cascade vs. add into
   the same project) produce clean `conflict` outcomes and a consistent
   database; parallel HTTP requests serialize without error.

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `plans/API.md` landed as the canonical HTTP contract and
      reconciled with shipped behavior at consolidation.
- [ ] `COMMANDS.md` documents `serve` and the `[serve] addr` config key
      (amending the single-key config statement).
- [ ] The deferred list above is re-homed durably (`MILESTONES.md`
      decisions or `API.md`) before this file is retired.
