package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

var desktopDriver desktop.Driver = desktop.MacDriver{}

func cmdDesktop(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	pos := positional(args)
	if len(pos) == 0 {
		fmt.Fprintln(stderr, "usage: hotseat desktop status|adopt|add|switch|recover")
		return 1
	}
	switch pos[0] {
	case "status":
		return cmdDesktopStatus(p, args, stdout, stderr)
	case "adopt":
		return cmdDesktopAdopt(p, pos[1:], stdout, stderr)
	case "switch":
		return cmdDesktopSwitch(p, args[1:], stdout, stderr)
	case "add":
		return cmdDesktopAdd(p, pos[1:], hasFlag(args, "--yes"), stdout, stderr)
	case "recover":
		unlock, err := desktop.Lock(p)
		if err != nil {
			return fail(stderr, err)
		}
		msg, err := desktop.Recover(p)
		unlock()
		if err != nil {
			return fail(stderr, err)
		}
		if msg == "" {
			msg = "nothing to recover"
		}
		return report(stdout, stderr, hasFlag(args, "--json"), map[string]string{"result": msg}, msg)
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
	if procs, err := desktopDriver.Procs(); err == nil {
		state := "not running"
		if len(desktop.Running(procs, p.Home)) > 0 {
			state = "running"
		}
		fmt.Fprintf(stdout, "Claude Desktop is %s\n", state)
	}
	if dirExists(p.DesktopStoreDir()) && fileExists(p.DesktopJournal()) {
		fmt.Fprintln(stdout, "an interrupted switch is pending; run: hotseat desktop recover")
		code = 1
	}
	if len(list) == 0 {
		fmt.Fprintln(stdout, "Claude Desktop is not managed yet; run: hotseat desktop adopt")
		return code
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

// desktopIdle errors unless every Claude Desktop process has exited.
func desktopIdle(p paths.Paths) error {
	procs, err := desktopDriver.Procs()
	if err != nil {
		return err
	}
	if left := desktop.Running(procs, p.Home); len(left) > 0 {
		return fmt.Errorf("Claude Desktop is running (pid %d); quit it first", left[0].PID)
	}
	return nil
}

func accountByUUID(reg account.Registry, uuid string) (account.Account, bool) {
	for _, a := range reg.Accounts {
		if a.AccountUUID == uuid && uuid != "" {
			return a, true
		}
	}
	return account.Account{}, false
}

// cmdDesktopAdopt starts managing Desktop data that already exists: with no
// arguments the data at the real path, or `<account> <dir>` to park a data
// dir that was logged in elsewhere (e.g. with --user-data-dir).
func cmdDesktopAdopt(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	unlock, err := desktop.Lock(p)
	if err != nil {
		return fail(stderr, err)
	}
	defer unlock()
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	s, err := desktop.LoadState(p)
	if err != nil {
		return fail(stderr, err)
	}
	switch len(args) {
	case 0:
		uuid, err := desktop.AccountUUID(p.DesktopDataDir())
		if err != nil {
			return fail(stderr, err)
		}
		a, ok := accountByUUID(reg, uuid)
		if !ok {
			return fail(stderr, fmt.Errorf("Claude Desktop is logged in to account %q, which is not registered; run hotseat init or hotseat add first", uuid))
		}
		if s.Active != "" && s.Active != a.Name {
			return fail(stderr, fmt.Errorf("Desktop is already managed with %s active", s.Active))
		}
		s.Active = a.Name
		s.Profiles[a.Name] = uuid
		if err := desktop.SaveState(p, s); err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintf(stdout, "Claude Desktop data at %s is now managed as %s\n", tildePath(p, p.DesktopDataDir()), a.Name)
		return 0
	case 2:
		name, dir := args[0], args[1]
		a, ok := reg.Find(name)
		if !ok {
			return fail(stderr, fmt.Errorf("unknown account %q", name))
		}
		if name == s.Active {
			return fail(stderr, fmt.Errorf("%s is the active Desktop account already", name))
		}
		if err := desktopIdle(p); err != nil {
			return fail(stderr, err)
		}
		uuid, err := desktop.AccountUUID(dir)
		if err != nil {
			return fail(stderr, err)
		}
		if uuid != a.AccountUUID {
			return fail(stderr, fmt.Errorf("%s is logged in to %q, not to %s (%s); not adopted", dir, uuid, name, a.AccountUUID))
		}
		target := p.DesktopProfileDir(name)
		if dirExists(target) {
			return fail(stderr, fmt.Errorf("%s already has Desktop data at %s", name, target))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fail(stderr, err)
		}
		if err := os.Rename(dir, target); err != nil {
			return fail(stderr, err)
		}
		s.Profiles[name] = uuid
		if err := desktop.SaveState(p, s); err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintf(stdout, "moved %s to %s; it is %s's Desktop data now\n", dir, tildePath(p, target), name)
		return 0
	default:
		fmt.Fprintln(stderr, "usage: hotseat desktop adopt [<account> <data-dir>]")
		return 1
	}
}

const desktopQuitTimeout = 20 * time.Second

func cmdDesktopSwitch(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	var to, resume string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--resume" && i+1 < len(args):
			resume = args[i+1]
			i++
		case args[i] == "--json":
		case to == "":
			to = args[i]
		default:
			fmt.Fprintln(stderr, "usage: hotseat desktop switch <account> [--resume current|<session-id>]")
			return 1
		}
	}
	if to == "" {
		fmt.Fprintln(stderr, "usage: hotseat desktop switch <account> [--resume current|<session-id>]")
		return 1
	}
	return switchDesktop(p, to, resume, hasFlag(args, "--json"), stdout, stderr)
}

func switchDesktop(p paths.Paths, to, resume string, jsonOut bool, stdout, stderr io.Writer) int {
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	res, err := desktop.Switch(p, reg, desktopDriver, to, desktop.SwitchOptions{Resume: resume, Timeout: desktopQuitTimeout})
	if err != nil {
		return fail(stderr, err)
	}
	if jsonOut {
		return writeJSON(stdout, stderr, res)
	}
	if res.Noop {
		fmt.Fprintf(stdout, "Claude Desktop already runs as %s\n", to)
		return 0
	}
	if res.From != res.To {
		fmt.Fprintf(stdout, "Claude Desktop: %s -> %s (%d sessions added, %d updated to the shared list)\n",
			res.From, res.To, res.Sessions.Copied, res.Sessions.Updated)
	}
	if res.Resumed != "" {
		fmt.Fprintf(stdout, "reopening session %s\n", res.Resumed)
	}
	return 0
}

func cmdDesktopAdd(p paths.Paths, args []string, yes bool, stdout, stderr io.Writer) int {
	var pos []string
	for _, a := range args {
		if a != "--yes" {
			pos = append(pos, a)
		}
	}
	args = pos
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: hotseat desktop add <account> [--yes]")
		return 1
	}
	if !yes {
		if err := desktopIdle(p); err != nil {
			return fail(stderr, fmt.Errorf("%v, or re-run with --yes to let Hotseat quit it", err))
		}
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	if _, ok := reg.Find(args[0]); !ok {
		return fail(stderr, fmt.Errorf("unknown account %q", args[0]))
	}
	if err := desktop.AddProfile(p, args[0]); err != nil {
		return fail(stderr, err)
	}
	if rc := switchDesktop(p, args[0], "", false, io.Discard, stderr); rc != 0 {
		return rc
	}
	fmt.Fprintf(stdout, "Claude Desktop opened with an empty profile for %s: log in to that account once; Hotseat checks it on the next switch\n", args[0])
	return 0
}
