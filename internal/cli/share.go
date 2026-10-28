package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/paths"
	"github.com/zahT3/roster/internal/share"
)

func cmdShare(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: roster share plan|set")
		return 1
	}
	switch args[0] {
	case "set":
		return cmdShareSet(p, args[1:], stdout, stderr)
	case "plan":
		return cmdSharePlan(p, args[1:], stdout, stderr)
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

func loadPlan(p paths.Paths) (account.Registry, share.Plan, error) {
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return reg, share.Plan{}, err
	}
	c, err := share.LoadClassifier(p)
	if err != nil {
		return reg, share.Plan{}, err
	}
	pl, err := share.MakePlan(p, reg, c)
	return reg, pl, err
}

func cmdSharePlan(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	_, pl, err := loadPlan(p)
	if err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, pl)
	}
	printPlan(pl, stdout)
	return 0
}

func printPlan(pl share.Plan, stdout io.Writer) {
	if len(pl.Actions) == 0 {
		fmt.Fprintln(stdout, "nothing to do: all accounts already share work data")
		return
	}
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	for _, a := range pl.Actions {
		acct := a.Account
		if acct == "" {
			acct = "(all)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", acct, a.Item, a.Kind, a.Detail)
	}
	tw.Flush()
	fmt.Fprintf(stdout, "%d conflicts will be skipped\n", pl.Conflicts())
}
