package main

import (
	"os"
	"path/filepath"

	"github.com/zahT3/roster/internal/cli"
)

func main() {
	if filepath.Base(os.Args[0]) == "claude" {
		// Shim mode is wired up in a later step.
		os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
