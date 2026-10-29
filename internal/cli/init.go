package cli

import (
	"fmt"
	"io"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

func cmdInit(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	found, err := account.Discover(p)
	if err != nil {
		return fail(stderr, err)
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	reg = mergeDiscovered(reg, found)
	if err := account.SaveRegistry(p, reg); err != nil {
		return fail(stderr, err)
	}
	cur, err := account.CurrentName(p)
	if err != nil {
		return fail(stderr, err)
	}
	if cur == "" {
		if def, ok := reg.Default(); ok {
			if err := account.SetCurrent(p, def.Name); err != nil {
				return fail(stderr, err)
			}
		}
	}
	if hasFlag(args, "--json") {
		cur, _ := account.CurrentName(p)
		return writeJSON(stdout, stderr, map[string]any{"current": cur, "accounts": reg.Accounts})
	}
	fmt.Fprintf(stdout, "registered %d account(s)\n", len(reg.Accounts))
	printAccounts(p, reg, stdout)
	fmt.Fprintln(stdout, "next: hotseat share plan")
	return 0
}

// mergeDiscovered keeps existing entries (and the names the user gave them)
// and appends newly found logins. Entries whose dir disappeared are kept so
// doctor can report them instead of silently forgetting an account.
func mergeDiscovered(reg account.Registry, found []account.Account) account.Registry {
	for _, f := range found {
		idx := -1
		for i, a := range reg.Accounts {
			if a.AccountUUID == f.AccountUUID {
				idx = i
				break
			}
		}
		if idx >= 0 {
			name := reg.Accounts[idx].Name
			reg.Accounts[idx] = f
			reg.Accounts[idx].Name = name
			continue
		}
		f.Name = uniqueName(reg, f.Name)
		reg.Accounts = append(reg.Accounts, f)
	}
	return reg
}

func uniqueName(reg account.Registry, name string) string {
	candidate := name
	for i := 2; ; i++ {
		if _, taken := reg.Find(candidate); !taken {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", name, i)
	}
}
