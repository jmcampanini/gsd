# gsd

Get shit done: a personal task manager kept in one SQLite database. A task lives in the inbox, in a project, or in an area; a project can move through the stages of a board; tags label tasks, projects, and areas; and the logbook lists what was done or cancelled. Every operation is a plain command with a `--json` mode, plus a read-only navigator (`gsd tui`) and a one-line capture prompt (`gsd capture`) for the terminal. No command runs another program or uses the network.


## Install

gsd is configured for versioned releases with prebuilt binaries for Linux and macOS on amd64 and arm64. The first release version has not been selected or published; until then, use the HEAD formula or build from source. The archive, mise, and shared-tap commands below apply once a stable release is available. The binary needs no external programs, including tmux; tmux is only a test dependency.


### Release archives

Download `gsd_<version>_<os>_<arch>.tar.gz` and `gsd_<version>_checksums.txt` from [GitHub Releases](https://github.com/jmcampanini/gsd/releases). Use `linux` or `darwin` for the OS and `amd64` or `arm64` for the architecture. Verify the archive against its SHA-256 checksum, extract it, and copy `gsd` to a directory on your `PATH`. Each archive also contains the README and license. GitHub build provenance can be verified with `gh attestation verify <archive> --repo jmcampanini/gsd`.

On macOS, after verifying the download, remove quarantine from the extracted binary before running it:

```sh
xattr -dr com.apple.quarantine ./gsd
```

To upgrade, repeat these steps with the archive for the new version and replace the installed binary.


### mise

With [mise](https://mise.jdx.dev) installed and activated in your shell:

```sh
mise use --global github:jmcampanini/gsd@latest
```

Upgrade to the latest stable release:

```sh
mise upgrade github:jmcampanini/gsd
```


### Homebrew

Install the released binary and bash, zsh, and fish completions from the shared tap:

```sh
brew install --cask jmcampanini/tap/gsd
```

Upgrade to the latest stable release:

```sh
brew update
brew upgrade --cask jmcampanini/tap/gsd
```

If switching from the HEAD formula, first run `brew uninstall --formula jmcampanini/gsd/gsd` to free the `gsd` binary path. Existing HEAD users can keep using the alternative below.


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

To upgrade a source installation, run `git pull --ff-only`, `mise install`, and `mise run build`, then replace the binary on your `PATH`.


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
