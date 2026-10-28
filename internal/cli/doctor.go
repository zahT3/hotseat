package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/doctor"
	"github.com/zahT3/roster/internal/paths"
)

func cmdDoctor(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	findings, err := doctor.Check(p, reg, os.Environ())
	if err != nil {
		return fail(stderr, err)
	}
	code := 0
	for _, f := range findings {
		if f.Level == "error" {
			code = 1
		}
	}
	if hasFlag(args, "--json") {
		if findings == nil {
			findings = []doctor.Finding{}
		}
		if rc := writeJSON(stdout, stderr, findings); rc != 0 {
			return rc
		}
		return code
	}
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "no problems found")
	}
	for _, f := range findings {
		acct := f.Account
		if acct == "" {
			acct = "-"
		}
		fmt.Fprintf(stdout, "[%s] %s %s %s: %s\n", f.Level, acct, f.Code, tildePath(p, f.Path), f.Message)
	}
	return code
}
