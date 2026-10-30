package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

// cmdNote sets the user's label for an account; no text clears it.
func cmdNote(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	pos := positional(args)
	if len(pos) == 0 {
		fmt.Fprintln(stderr, "usage: hotseat note <account> [text...]")
		return 1
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	name, note := pos[0], strings.TrimSpace(strings.Join(pos[1:], " "))
	idx := -1
	for i, a := range reg.Accounts {
		if a.Name == name {
			idx = i
		}
	}
	if idx < 0 {
		return fail(stderr, fmt.Errorf("unknown account %q", name))
	}
	reg.Accounts[idx].Note = note
	if err := account.SaveRegistry(p, reg); err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, map[string]string{"account": name, "note": note})
	}
	if note == "" {
		fmt.Fprintf(stdout, "note for %s cleared\n", name)
	} else {
		fmt.Fprintf(stdout, "note for %s: %s\n", name, note)
	}
	return 0
}
