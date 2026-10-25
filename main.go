package main

import (
	"os"

	"github.com/jmcampanini/gsd/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
