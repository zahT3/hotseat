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
	reg, skipped := mergeDiscovered(reg, found)
	for _, w := range skipped {
		fmt.Fprintln(stderr, "hotseat: "+w)
	}
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
//
// A directory registered to one account but now logged in to another is
// left out, with a warning: registering it again would give two names to
// one directory, and it must not keep the other logins from registering.
func mergeDiscovered(reg account.Registry, found []account.Account) (account.Registry, []string) {
	var skipped []string
	for _, f := range found {
		idx, taken := -1, ""
		for i, a := range reg.Accounts {
			if a.AccountUUID != f.AccountUUID && account.SameConfigDir(a.ConfigDir, f.ConfigDir) {
				taken = a.Name
			}
			if a.AccountUUID == f.AccountUUID {
				idx = i
			}
		}
		if taken != "" {
			skipped = append(skipped, changedLoginWarning(reg, taken, f.ConfigDir))
			continue
		}
		if idx >= 0 {
			kept := reg.Accounts[idx]
			reg.Accounts[idx] = f
			reg.Accounts[idx].Name, reg.Accounts[idx].Note, reg.Accounts[idx].Icon = kept.Name, kept.Note, kept.Icon
			continue
		}
		f.Name = uniqueName(reg, f.Name)
		reg.Accounts = append(reg.Accounts, f)
	}
	return reg, skipped
}

func changedLoginWarning(reg account.Registry, name, dir string) string {
	msg := fmt.Sprintf("skipped %s: it is registered to %s but now has a different login; log back in to %s there", dir, name, name)
	if a, ok := reg.Find(name); ok && !a.IsDefault {
		msg += fmt.Sprintf(", or drop %s from Hotseat with: hotseat remove %s --forget --yes", name, name)
	}
	return msg
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
