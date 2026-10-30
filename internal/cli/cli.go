// Package cli implements the hotseat subcommands.
package cli

import (
	"fmt"
	"io"

	"github.com/zahT3/hotseat/internal/paths"
)

const Version = "0.1.0"

type command func(p paths.Paths, args []string, stdout, stderr io.Writer) int

var commands = map[string]command{
	"init":       cmdInit,
	"accounts":   cmdAccounts,
	"use":        cmdUse,
	"current":    cmdCurrent,
	"share":      cmdShare,
	"doctor":     cmdDoctor,
	"add":        cmdAdd,
	"shim":       cmdShim,
	"statusline": cmdStatusline,
	"hook":       cmdHook,
	"handoff":    cmdHandoff,
	"usage":      cmdUsage,
	"desktop":    cmdDesktop,
	"panel":      cmdPanel,
	"note":       cmdNote,
	"setup":      cmdSetup,
	"uninstall":  cmdUninstall,
	"icon":       cmdIcon,
	"remove":     cmdRemove,
}

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: hotseat <command> [args]")
		return 1
	}
	if args[0] == "version" {
		if hasFlag(args, "--json") {
			return writeJSON(stdout, stderr, map[string]string{"version": Version})
		}
		fmt.Fprintf(stdout, "hotseat %s\n", Version)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "hotseat: unknown command %q\n", args[0])
		return 1
	}
	p, err := paths.FromEnv()
	if err != nil {
		return fail(stderr, err)
	}
	return cmd(p, args[1:], stdout, stderr)
}
