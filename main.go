// Command gsd is a personal task manager kept in one SQLite database. It runs
// plain subcommands with an optional --json mode plus a read-only terminal
// navigator and a one-line capture prompt.
package main

import (
	"os"

	"github.com/jmcampanini/gsd/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
