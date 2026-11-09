package main

import (
	"os"

	"github.com/ContractorKeith/slowbooks-cli/internal/cli"
)

var version = "dev"
var commit = "unknown"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, cli.Build{Version: version, Commit: commit}))
}
