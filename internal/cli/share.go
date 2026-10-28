package cli

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/paths"
	"github.com/zahT3/roster/internal/share"
)

func cmdShare(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: roster share plan|apply|rollback|backups|set")
		return 1
	}
	switch args[0] {
	case "set":
		return cmdShareSet(p, args[1:], stdout, stderr)
	case "plan":
		return cmdSharePlan(p, args[1:], stdout, stderr)
	case "apply":
		return cmdShareApply(p, args[1:], stdout, stderr)
	case "rollback":
		return cmdShareRollback(p, args[1:], stdout, stderr)
	case "backups":
		return cmdShareBackups(p, args[1:], stdout, stderr)
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
	for _, s := range pl.Skipped {
		fmt.Fprintf(stdout, "skipped %s\n", s)
	}
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

func cmdShareApply(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	reg, pl, err := loadPlan(p)
	if err != nil {
		return fail(stderr, err)
	}
	m, err := share.Apply(p, reg, pl, share.ApplyOptions{Force: hasFlag(args, "--force")})
	if errors.Is(err, share.ErrClaudeRunning) {
		fmt.Fprintf(stderr, "roster: %v\nclose every claude session (CLI and Desktop) and retry, or pass --force\n", err)
		return 2
	}
	if err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, m)
	}
	for _, s := range m.Steps {
		fmt.Fprintf(stdout, "done  %s %s %s\n", s.Action.Account, s.Action.Item, s.Action.Kind)
		for _, c := range s.Conflicts {
			fmt.Fprintf(stdout, "      kept shared copy %s\n", tildePath(p, c))
			if rel, err := filepath.Rel(s.Action.Target, c); err == nil && s.BackupPath != "" {
				fmt.Fprintf(stdout, "      account copy saved at %s\n", tildePath(p, filepath.Join(s.BackupPath, rel)))
			}
		}
	}
	fmt.Fprintf(stdout, "%d conflicts skipped; run `roster doctor` to review\n", pl.Conflicts())
	fmt.Fprintf(stdout, "backup %s; undo with: roster share rollback %s\n", m.ID, m.ID)
	return 0
}

func cmdShareRollback(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	var id string
	for _, a := range args {
		if a != "--force" {
			id = a
		}
	}
	if id == "" {
		fmt.Fprintln(stderr, "usage: roster share rollback <backup-id> [--force]   (list ids with: roster share backups)")
		return 1
	}
	moved, err := share.Rollback(p, id, share.RollbackOptions{Force: hasFlag(args, "--force")})
	for _, m := range moved {
		fmt.Fprintf(stdout, "changed since the apply, moved aside (not deleted): %s\n", tildePath(p, m))
	}
	if errors.Is(err, share.ErrClaudeRunning) {
		fmt.Fprintf(stderr, "roster: %v\nclose every claude session (CLI and Desktop) and retry, or pass --force\n", err)
		return 2
	}
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "rolled back %s\n", id)
	return 0
}

func cmdShareBackups(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	ids, err := share.ListBackups(p)
	if err != nil {
		return fail(stderr, err)
	}
	for _, id := range ids {
		m, err := share.LoadManifest(p, id)
		if err != nil {
			fmt.Fprintf(stdout, "%s  (unreadable: %v)\n", id, err)
			continue
		}
		state := ""
		if m.RolledBack {
			state = "  rolled back"
		}
		fmt.Fprintf(stdout, "%s  %d steps%s\n", id, len(m.Steps), state)
	}
	return 0
}
