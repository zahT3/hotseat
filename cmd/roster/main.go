package main

import (
	"os"
	"path/filepath"

	"github.com/zahT3/roster/internal/cli"
	"github.com/zahT3/roster/internal/shim"
)

func main() {
	if filepath.Base(os.Args[0]) == "claude" {
		shim.Run(os.Args)
		return
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
