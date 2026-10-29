package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

func cmdAccounts(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	cur, err := account.CurrentName(p)
	if err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		accts := reg.Accounts
		if accts == nil {
			accts = []account.Account{}
		}
		return writeJSON(stdout, stderr, map[string]any{"current": cur, "accounts": accts})
	}
	printAccounts(p, reg, stdout)
	return 0
}

func printAccounts(p paths.Paths, reg account.Registry, stdout io.Writer) {
	cur, _ := account.CurrentName(p)
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	for _, a := range reg.Accounts {
		mark := " "
		if a.Name == cur {
			mark = "*"
		}
		fmt.Fprintf(tw, "%s %s\t%s\t%s\n", mark, a.Name, tildePath(p, a.ConfigDir), shortID(a.AccountUUID))
	}
	tw.Flush()
}

func tildePath(p paths.Paths, path string) string {
	if path == p.Home || strings.HasPrefix(path, p.Home+"/") {
		return "~" + strings.TrimPrefix(path, p.Home)
	}
	return path
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func writeJSON(stdout, stderr io.Writer, v any) int {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, string(b))
	return 0
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// positional drops the --json and --force flags so commands can read their
// arguments in any order.
func positional(args []string) []string {
	var out []string
	for _, a := range args {
		if a != "--json" && a != "--force" {
			out = append(out, a)
		}
	}
	return out
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "hotseat: %v\n", err)
	return 1
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
