package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

// cmdIcon sets the symbol shown for an account (an SF Symbol name such as
// briefcase.fill, or an emoji); no symbol clears it.
func cmdIcon(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	pos := positional(args)
	if len(pos) == 0 || len(pos) > 2 {
		fmt.Fprintln(stderr, "usage: hotseat icon <account> [sf-symbol-name|emoji]")
		return 1
	}
	icon := ""
	if len(pos) == 2 {
		icon = pos[1]
	}
	if strings.ContainsAny(icon, " \t\n") || utf8.RuneCountInString(icon) > 40 {
		return fail(stderr, fmt.Errorf("%q is not a symbol name or emoji", icon))
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	idx := -1
	for i, a := range reg.Accounts {
		if a.Name == pos[0] {
			idx = i
		}
	}
	if idx < 0 {
		return fail(stderr, fmt.Errorf("unknown account %q", pos[0]))
	}
	reg.Accounts[idx].Icon = icon
	if err := account.SaveRegistry(p, reg); err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, map[string]string{"account": pos[0], "icon": icon})
	}
	fmt.Fprintf(stdout, "icon for %s: %s\n", pos[0], icon)
	return 0
}
