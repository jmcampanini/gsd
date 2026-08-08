# gsd

Get shit done — a personal task CLI backed by SQLite.

## Install

### Homebrew `--HEAD`

```sh
brew tap jmcampanini/gsd https://github.com/jmcampanini/gsd
brew install --HEAD jmcampanini/gsd/gsd
brew upgrade --fetch-HEAD gsd
```

### Source/dev build fallback

A source build requires Go (see `go.mod`). The development and e2e test
workflow also requires `tmux`.

```sh
make build
# binary at ./build/gsd
```

## Verify

With `tmux` installed:

```sh
make check
```

<!-- cli-standards: CLI-DOCS-001, CLI-RELEASE-003 -->
