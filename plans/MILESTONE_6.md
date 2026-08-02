# Milestone 6 — Config

Data mode: throwaway. Depends on: Milestone 5.

## Capability

gsd implements the canonical v1 config contract from `COMMANDS.md`: a TOML
config file loaded through go-config-loader with full precedence and
provenance, a `gsd config` report command, and proper color modes. This lands *before*
go-live so the question "where does my real data live" is answered by
config, permanently, before real data exists.

## Schema delta

- The tag aggregates inside the three views change from `ORDER BY g.id`
  to `ORDER BY g.title COLLATE NOCASE`, per the reconciled `SCHEMA.md`:
  entity `tags` arrays become alphabetical, matching `tags list`. No
  table changes.
- `user_version` → `9006`.

## Config surface (deliberately minimal)

Keys, with their full precedence chain (low → high:
defaults → config file → env → flags):

| Key | TOML | Env | Flag | Default |
|-----|------|-----|------|---------|
| db path | `db_path` | `GSD_DB` | `--db PATH` | `$XDG_DATA_HOME/gsd/gsd.db` |
| color | `color` | `GSD_COLOR` | `--color=auto\|always\|never` | `auto` |

- File location: `$XDG_CONFIG_HOME/gsd/config.toml`; discovered file is
  optional, but an explicit `--config PATH` that can't load fails loud.
  Per CLI-CONFIG-002, the discovered-file behavior gets documented as
  currently implemented while go-config-loader #13 leaves the
  optional-directory contract open.
- One field, two tags, each owning a namespace: `toml:"db_path"` names
  the file key; `config:"db"` names env and flag (`GSD_DB` / `--db`), so
  `COMMANDS.md`'s existing env/flag contract is preserved verbatim.
- New keys require a demonstrated need — every key is permanent API.

## Commands

```text
gsd config                # valid, redirectable TOML of effective config
gsd config --provenance   # adds per-field source (default/file/env/flag)
gsd --config PATH ...     # explicit config file (required if given)
gsd --color=auto|always|never ...
```

Color resolution (CLI-OUTPUT-001/002): explicit `--color` flag > nonempty
`NO_COLOR` > env/file value > destination-aware auto-detection, evaluated
per stream; `TERM=dumb` disables in auto. `--json` output never carries
ANSI regardless (CLI-OUTPUT-003).

No secrets exist in this surface; redaction hooks are structurally present
via configreporter but redact nothing in v1 (CLI-CONFIG-004 satisfied
trivially — noted so the review checks the structure, not the absence).

## Chunks

0. **Milestone 5 consolidation** — the structural decisions from the
   Milestone 5 foundation review, applied in one chunk before Config
   work begins (see below).
1. **Loader integration** — config struct, defaults, file/env/flag
   loaders in precedence order, `--config`, db-path resolution moved onto
   the loaded config; existing `--db`/`GSD_DB` behavior proven unchanged
   by e2e.
2. **Report + color** — `gsd config` (+ `--provenance`) via
   configreporter, `--color` flag and full resolution chain wired into
   lipgloss profile selection.

### Chunk 0: Milestone 5 consolidation

Behavior-preserving except for the tag-ordering change in the schema
delta above. The manifest:

- **Root/core store split.** Statement-issuing logic moves to an
  executor-bound core type per store; the root type holds only the
  database handle and makes each verb's atomicity decision as a visible
  one-line wrapper (pool-bound core for single-statement verbs,
  `runInTransaction` for multi-statement ones). The `database == nil`
  mode flag and every `if s.database != nil` preamble disappear, as
  does `Tasks.List`'s hand-synced wrap predicate: filtered lists always
  run in a deferred read transaction. Acceptance check:
  `applyTransition` retains a single action switch — the re-entry
  switch dies with the dual mode.
- **Read-then-delete.** The six `WITH snapshot AS MATERIALIZED` delete
  statements become tag-enriched finds/listings followed by plain
  deletes inside the existing immediate-transaction machinery. Same
  observable behavior; the pre-CASCADE snapshot trick is deleted, not
  documented.
- **Service-owned tag resolution on the read path.** `list --tag`
  resolves the tag name in the service (as every write path already
  does) and passes the resolved tag ID to the store; the store filter
  loses its resolution half. One layer owns the unknown-tag rule.
- **Sibling alignment.** The three tag service flows and the three cmd
  tagging factories become byte-parallel modulo entity nouns; cmd
  naming standardizes on `<noun>TaggingMutation` /
  `new<Noun>TaggingCommand`.
- **Tag delete drops its redundant leading `Find`**; the sequence
  assertion narrows to the semantic pair (`CountUsage` before
  `Delete`).
- **Shared `tagUsageCountExpression`** replaces the duplicated
  three-join-table sum in `Tags.List` and `Tags.CountUsage`.
- **`domain.TagNames`** — a `[]string` whose `MarshalJSON` emits `[]`
  for nil — becomes the `Tags` field type in task, project, area, and
  logbook, making `"tags": null` structurally impossible. Content
  correctness stays owned by per-read-path store tests.
- **`collectRows[T]`** replaces the five hand-rolled rows→slice
  collectors.
- **Alphabetical tag arrays.** Views and `tagJSONExpression` move to
  `ORDER BY g.title COLLATE NOCASE` per the schema delta;
  order-asserting tests updated; schema convergence audit re-run.

Deliberately not in this chunk: genericizing the intentionally-parallel
tag service flows (revisit on the first sibling-divergence bug or a
post-v1 attach-semantics change) and a typed transition spec for
`applyTransition` (revisit if post-v1 work adds transitions).

## User stories

### Your database location is a setting, not a habit

```text
$ cat ~/.config/gsd/config.toml
db_path = "/Users/jmcampanini/Sync/gsd/gsd.db"
$ gsd inbox            # uses the synced path, no flags
```

### You can always see what config is in effect, and why

```text
$ gsd config
db_path = "/Users/jmcampanini/Sync/gsd/gsd.db"
color = "auto"
$ GSD_COLOR=never gsd config --provenance
db_path = ...   # file: ~/.config/gsd/config.toml
color = "never" # env: GSD_COLOR
```

### Redirected output is clean without asking

```text
$ gsd inbox > inbox.txt        # no ANSI in the file
$ NO_COLOR=1 gsd inbox         # plain even on a TTY
$ gsd inbox --color=always | less -R   # styled on purpose
```

## Agent-verified end-to-end workflow

1. Precedence matrix for `db_path`: default only; file; file+env;
   file+env+flag — each layer wins over the previous, verified by
   `gsd config --provenance` and by where the db file actually appears.
2. `gsd config` output round-trips: feed it back as `--config`, effective
   config identical.
3. Explicit `--config /nonexistent` fails loud, exit 1; absent discovered
   file is silently fine.
4. Color: `--json` output has no ANSI under `--color=always`; `NO_COLOR`
   beats file/env `color=always`; `--color=always` beats `NO_COLOR`.
5. `make check` proves existing e2e workflows still pass with config in
   the loading path (no behavior regressions).

## Exit criteria

Standard exit workflow (see [`PROCESS.md`](PROCESS.md)), plus:

- [ ] `COMMANDS.md` § Configuration and § Database rechecked against the
      shipped precedence, report, and color behavior before Go live starts.
- [ ] **Schema convergence audit re-run** after the ordering change:
      dev database schema byte-comparable to `SCHEMA.md`.

## Standards

CLI-CONFIG-001/002/003/004, CLI-OUTPUT-001/002/003, CLI-DOCS-002.
