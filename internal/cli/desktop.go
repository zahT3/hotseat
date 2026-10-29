package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

func cmdDesktop(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	pos := positional(args)
	if len(pos) == 0 {
		fmt.Fprintln(stderr, "usage: hotseat desktop status|adopt|add|switch|recover")
		return 1
	}
	switch pos[0] {
	case "status":
		return cmdDesktopStatus(p, args, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "hotseat: unknown desktop command %q\n", pos[0])
		return 1
	}
}

type profileStatus struct {
	Account  string `json:"account"`
	Active   bool   `json:"active"`
	Dir      string `json:"dir"`
	Present  bool   `json:"present"`
	Recorded string `json:"recordedUuid"`
	Actual   string `json:"actualUuid"`
	Expected string `json:"expectedUuid"`
	Problem  string `json:"problem,omitempty"`
}

func desktopStatus(p paths.Paths) ([]profileStatus, error) {
	s, err := desktop.LoadState(p)
	if err != nil {
		return nil, err
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(s.Profiles))
	for n := range s.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []profileStatus
	for _, n := range names {
		ps := profileStatus{Account: n, Active: n == s.Active, Dir: desktop.ProfileDir(p, s, n), Recorded: s.Profiles[n]}
		if a, ok := reg.Find(n); ok {
			ps.Expected = a.AccountUUID
		}
		actual, err := desktop.AccountUUID(ps.Dir)
		ps.Present = err == nil && dirExists(ps.Dir)
		ps.Actual = actual
		switch {
		case !ps.Present:
			ps.Problem = "data dir missing"
		case ps.Actual == "" && ps.Recorded == "":
			ps.Problem = "not logged in yet"
		case ps.Expected != "" && ps.Actual != "" && ps.Actual != ps.Expected:
			ps.Problem = "mismatch: logged in to a different Claude account than " + n
		case ps.Recorded != "" && ps.Actual != ps.Recorded:
			ps.Problem = "mismatch: account changed since Hotseat last saw it"
		}
		out = append(out, ps)
	}
	return out, nil
}

func cmdDesktopStatus(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	list, err := desktopStatus(p)
	if err != nil {
		return fail(stderr, err)
	}
	code := 0
	for _, ps := range list {
		if strings.HasPrefix(ps.Problem, "mismatch") || ps.Problem == "data dir missing" {
			code = 1
		}
	}
	if hasFlag(args, "--json") {
		if list == nil {
			list = []profileStatus{}
		}
		if rc := writeJSON(stdout, stderr, list); rc != 0 {
			return rc
		}
		return code
	}
	if len(list) == 0 {
		fmt.Fprintln(stdout, "Claude Desktop is not managed yet; run: hotseat desktop adopt")
		return 0
	}
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	for _, ps := range list {
		mark := " "
		if ps.Active {
			mark = "*"
		}
		state := "ok"
		if ps.Problem != "" {
			state = ps.Problem
		}
		fmt.Fprintf(tw, "%s %s\t%s\t%s\n", mark, ps.Account, tildePath(p, ps.Dir), state)
	}
	tw.Flush()
	return code
}
