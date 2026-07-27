# Milestone 6 — Config

Data mode: throwaway (last one). Depends on: Milestone 5.

## Capability

gsd becomes a configured CLI (the v1 divergence, D-001): a TOML config
file loaded through go-config-loader with full precedence and provenance,
a `gsd config` report command, and proper color modes. This lands *before*
go-live so the question "where does my real data live" is answered by
config, permanently, before real data exists.

## Schema delta

None — `user_version` unchanged. Milestone 5's converged schema is
exactly what Milestone 7 baselines.

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

1. **Loader integration** — config struct, defaults, file/env/flag
   loaders in precedence order, `--config`, db-path resolution moved onto
   the loaded config; existing `--db`/`GSD_DB` behavior proven unchanged
   by e2e.
2. **Report + color** — `gsd config` (+ `--provenance`) via
   configreporter, `--color` flag and full resolution chain wired into
   lipgloss profile selection.

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
5. `make check` proves prior milestones' e2e still pass with config in
   the loading path (no behavior regressions).

## Exit criteria

Standard exit criteria (see `MILESTONES.md`), plus:

- [ ] D-001 consolidated: `COMMANDS.md` § Database rewritten (precedence
      including config file, config section added) — due before
      Milestone 7 starts.

## Standards

CLI-CONFIG-001/002/003/004, CLI-OUTPUT-001/002/003, CLI-DOCS-002.
