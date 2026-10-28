// Package cli implements the roster subcommands.
package cli

import (
	"fmt"
	"io"

	"github.com/zahT3/roster/internal/paths"
)

const Version = "0.1.0"

type command func(p paths.Paths, args []string, stdout, stderr io.Writer) int

var commands = map[string]command{
	"init":     cmdInit,
	"accounts": cmdAccounts,
	"use":      cmdUse,
	"current":  cmdCurrent,
	"share":    cmdShare,
}

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: roster <command> [args]")
		return 1
	}
	if args[0] == "version" {
		fmt.Fprintf(stdout, "roster %s\n", Version)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "roster: unknown command %q\n", args[0])
		return 1
	}
	p, err := paths.FromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	return cmd(p, args[1:], stdout, stderr)
}
