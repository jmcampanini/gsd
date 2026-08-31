# gsd

Get shit done: a personal task manager kept in one SQLite database. A task lives in the inbox, in a project, or in an area; a project can move through the stages of a board; tags label tasks, projects, and areas; and the logbook lists what was done or cancelled. Every operation is a plain command with a `--json` mode, plus a read-only navigator (`gsd tui`) and a one-line capture prompt (`gsd capture`) for the terminal.

Command help is the canonical reference: `gsd --help` and each command's `--help` describe every user-facing contract, `gsd config --help` describes where the database and configuration are found, and `gsd help exit-codes` describes exit statuses.

## Install

gsd distributes from HEAD only; there is no release channel or tagged binary.

### Homebrew

```sh
brew tap jmcampanini/gsd https://github.com/jmcampanini/gsd
brew install --HEAD jmcampanini/gsd/gsd
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD gsd
```

### From source

A source build requires Go (see `go.mod`).

```sh
make build
# then copy ./build/gsd to a directory on your PATH
```

## Representative commands

| Command | Result |
|---|---|
| `gsd add "Buy milk" --due tomorrow` | Add an open inbox task with a due date. |
| `gsd available` | List every task that can be worked on now. |
| `gsd done 7` | Complete task 7, promoting its project when the task promotes. |
| `gsd projects add "Move house" --area 2 --board Home` | Add a project filed under area 2 and placed on the first stage of board Home. |
| `gsd project move 3 Doing` | Move project 3 to the Doing stage of its board. |
| `gsd areas add Work` | Add an area. |
| `gsd boards add Home --stage Todo --stage Doing --stage Done` | Add a board with three stages. |
| `gsd tags add errands` | Create a tag; `gsd tag 7 errands` then attaches it to task 7. |
| `gsd search "milk OR bread" --json` | Search titles, tags, and notes and print the hits as JSON. |
| `gsd logbook` | List done and cancelled tasks and projects. |
| `gsd tui` | Browse everything in the terminal without changing it. |
| `gsd config --provenance` | Print the effective configuration and where each value came from. |

## Required external programs

None. gsd runs no other program and uses no network.

## Database and configuration

The database is one SQLite file. It defaults to `$XDG_DATA_HOME/gsd/gsd.db` (`~/.local/share/gsd/gsd.db` when `XDG_DATA_HOME` is unset) and can be moved with `db_path` in `$XDG_CONFIG_HOME/gsd/config.toml` (`~/.config/gsd/config.toml` when unset), the `GSD_DB` environment variable, or `--db PATH`, each overriding the one before; `--config PATH` names a config file explicitly. `gsd config --help` documents the full precedence and `gsd config` prints the value in effect.

```toml
# ~/.config/gsd/config.toml
db_path = "/Users/me/Sync/gsd.db"
```

## Verify

`make check` runs the complete local verification contract. Its end-to-end tests drive the built binary through `tmux`, which must be on `PATH`.
