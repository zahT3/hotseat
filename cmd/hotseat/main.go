package main

import (
	"os"
	"path/filepath"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/shim"
)

func main() {
	if filepath.Base(os.Args[0]) == "claude" {
		shim.Run(os.Args)
		return
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
