package cli

import (
	"fmt"
	"io"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

func cmdUse(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: hotseat use <account>")
		return 1
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	if _, ok := reg.Find(args[0]); !ok {
		fmt.Fprintf(stderr, "hotseat: unknown account %q\n", args[0])
		return 1
	}
	if err := account.SetCurrent(p, args[0]); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "current account: %s (new claude sessions use it; running ones keep theirs)\n", args[0])
	return 0
}

func cmdCurrent(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	cur, err := account.CurrentName(p)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, cur)
	return 0
}
