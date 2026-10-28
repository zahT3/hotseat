// Package cli implements the roster subcommands.
package cli

import (
	"fmt"
	"io"
)

const Version = "0.1.0"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: roster <command> [args]")
		return 1
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "roster %s\n", Version)
		return 0
	default:
		fmt.Fprintf(stderr, "roster: unknown command %q\n", args[0])
		return 1
	}
}
