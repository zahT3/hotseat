package cli

import (
	"fmt"
	"io"

	"github.com/zahT3/roster/internal/paths"
	"github.com/zahT3/roster/internal/share"
)

func cmdShare(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: roster share set <item> shared|local")
		return 1
	}
	switch args[0] {
	case "set":
		return cmdShareSet(p, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "roster: unknown share command %q\n", args[0])
		return 1
	}
}

func cmdShareSet(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: roster share set <item> shared|local")
		return 1
	}
	if err := share.SetOverride(p, args[0], share.Class(args[1])); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "%s is now %s\n", args[0], args[1])
	return 0
}
