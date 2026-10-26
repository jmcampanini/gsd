# gsd

Get shit done: a personal task manager kept in one SQLite database. A task lives in the inbox, in a project, or in an area; a project can move through the stages of a board; tags label tasks, projects, and areas; and the logbook lists what was done or cancelled. Every operation is a plain command with a `--json` mode, plus a read-only navigator (`gsd tui`) and a one-line capture prompt (`gsd capture`) for the terminal. No command runs another program or uses the network.


## Install

gsd is configured for versioned releases for Linux and macOS on amd64 and arm64. The first release version has not been selected or published; until then, use the HEAD formula or build from source. The binary needs no external programs; tmux is only a test dependency.

Once a stable release is available, install or upgrade with mise or the shared Homebrew tap:

```sh
mise use -g github:jmcampanini/gsd        # Install with mise
mise upgrade github:jmcampanini/gsd      # Upgrade with mise
brew install --cask jmcampanini/tap/gsd  # Install with Homebrew
brew upgrade --cask jmcampanini/tap/gsd  # Upgrade with Homebrew
```

If switching from the HEAD formula, first run `brew uninstall --formula jmcampanini/gsd/gsd` to free the `gsd` binary path. The cask includes bash, zsh, and fish completions.

Or download an archive from [GitHub Releases](https://github.com/jmcampanini/gsd/releases), extract it, and put `gsd` on your `PATH`; to upgrade, replace it with the new release binary. Releases include SHA-256 checksums and build provenance attestations. On macOS, a browser download needs `xattr -d com.apple.quarantine gsd` before it runs.


### Homebrew HEAD alternative

Build the latest commit from source with the existing repository tap:

```sh
brew tap jmcampanini/gsd https://github.com/jmcampanini/gsd
brew install --HEAD jmcampanini/gsd/gsd
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD gsd
```


### From source

With Git and [mise](https://mise.jdx.dev) installed (`brew install mise` on macOS), mise supplies the pinned Go toolchain from `mise.toml`:

```sh
git clone https://github.com/jmcampanini/gsd
cd gsd
mise trust
mise install
mise run build
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

The canonical reference is `gsd --help`, each command's `--help`, and the help topic `gsd help exit-codes`.


## Configuration

The config file is discovered at `$XDG_CONFIG_HOME/gsd/config.toml` (`~/.config/gsd/config.toml` when `XDG_CONFIG_HOME` is unset). The database defaults to `$XDG_DATA_HOME/gsd/gsd.db` (`~/.local/share/gsd/gsd.db` when `XDG_DATA_HOME` is unset).

See `gsd config --help` for configuration precedence and the file format.

```toml
# ~/.config/gsd/config.toml
db_path = "/Users/me/Sync/gsd.db"
```
