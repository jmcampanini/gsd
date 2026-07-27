# Divergence Log

Departures from `OVERVIEW.md` / `COMMANDS.md` / `SCHEMA.md`, recorded as
they happen. Before each new milestone starts, open entries are
consolidated — the spec docs get updated to match decided reality — and the
entry moves to Closed with a note of what changed.

Entry format:

```text
## D-NNN: short title
- Recorded: date, during <milestone/planning>
- Diverges from: <doc> § <section>
- Decision: what we're doing instead, and why
- Consolidate by: start of <milestone>
- Consolidation: which doc sections must change
```

## Open

### D-001: config file ships in v1

- Recorded: 2026-07-26, during milestone planning
- Diverges from: `COMMANDS.md` § Database ("No config file in v1")
- Decision: v1 includes a config file loaded via go-config-loader, landing
  in Milestone 6 so the database location story is settled before any
  non-throwaway data exists. Full new surface: TOML config file at
  `$XDG_CONFIG_HOME/gsd/config.toml` with keys `db_path` and `color`;
  `--config PATH` (explicit file, required if given); `gsd config` report
  command with `--provenance`; `--color=auto|always|never` flag and
  `GSD_COLOR` env; color resolution `--color` > `NO_COLOR` > env/file >
  auto-detection.
- Consolidate by: start of Milestone 7
- Consolidation: rewrite `COMMANDS.md` § Database precedence to
  flag > env > config file > default; add a Config section covering the
  file location, keys, the report command, and color modes.

## Closed

### D-002: TUI is out of the v1 milestone map

- Recorded: 2026-07-26, during milestone planning
- Diverges from: `COMMANDS.md` § TUI (presented as one of v1's surfaces
  with full parity)
- Decision: the TUI is a separate post-v1 effort. The CLI grammar and
  shared-core design in the spec stay as the target so nothing in v1
  forecloses it, but no TUI work is scheduled in this map.
- Consolidate by: start of Milestone 0
- Consolidation: completed 2026-07-27. `COMMANDS.md` now identifies the CLI
  as the v1 surface and marks the planned TUI as post-v1.
