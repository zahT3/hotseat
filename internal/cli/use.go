package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

func cmdUse(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	jsonOut := hasFlag(args, "--json")
	cliOnly, yes := hasFlag(args, "--cli-only"), hasFlag(args, "--yes")
	var pos []string
	for _, a := range positional(args) {
		if a != "--cli-only" && a != "--yes" {
			pos = append(pos, a)
		}
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: hotseat use <account> [--cli-only] [--yes]")
		return 1
	}
	name := pos[0]
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	if _, ok := reg.Find(name); !ok {
		fmt.Fprintf(stderr, "hotseat: unknown account %q\n", name)
		return 1
	}
	if err := account.SetCurrent(p, name); err != nil {
		return fail(stderr, err)
	}
	out := map[string]any{"current": name}
	text := []string{fmt.Sprintf("current account: %s (new claude sessions use it; running ones keep theirs)", name)}
	code := 0
	if !cliOnly {
		code = useDesktop(p, name, yes, out, &text, stderr)
	}
	if code == 1 {
		return 1
	}
	if jsonOut {
		if rc := writeJSON(stdout, stderr, out); rc != 0 {
			return rc
		}
		return code
	}
	fmt.Fprintln(stdout, strings.Join(text, "\n"))
	return code
}

// useDesktop moves Claude Desktop to the account `hotseat use` picked. A
// closed Desktop is switched silently (the next launch is the new account);
// a running one is quit and reopened on the current session only with --yes.
func useDesktop(p paths.Paths, name string, yes bool, out map[string]any, text *[]string, stderr io.Writer) int {
	s, err := desktop.LoadState(p)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if s.Active == "" {
		return 0
	}
	if s.Active == name {
		// Already the Desktop account on record; make sure the data at the
		// real path still agrees before saying nothing.
		reg, err := account.LoadRegistry(p)
		if err != nil {
			fail(stderr, err)
			return 1
		}
		if _, err := desktop.Switch(p, reg, desktopDriver, name, desktop.SwitchOptions{Timeout: desktopQuitTimeout}); err != nil {
			fail(stderr, err)
			return 1
		}
		return 0
	}
	if _, ok := s.Profiles[name]; !ok {
		hint := fmt.Sprintf("Claude Desktop has no data for %s yet; run: hotseat desktop add %s", name, name)
		out["desktopHint"] = hint
		*text = append(*text, hint)
		return 0
	}
	procs, err := desktopDriver.Procs()
	if err != nil {
		fail(stderr, err)
		return 1
	}
	running := len(desktop.Running(procs, p.Home)) > 0
	if running && !yes {
		msg := fmt.Sprintf("Claude Desktop is running as %s. Re-run with --yes to quit it and reopen it as %s on the session you were in.", s.Active, name)
		out["desktopNeedsYes"] = true
		*text = append(*text, msg)
		return 3
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	opts := desktop.SwitchOptions{Timeout: desktopQuitTimeout, NoLaunch: !running}
	if running {
		opts.Resume = "current"
	}
	res, err := desktop.Switch(p, reg, desktopDriver, name, opts)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	out["desktop"] = res
	line := fmt.Sprintf("Claude Desktop: %s -> %s", res.From, res.To)
	if !running {
		line += " (it opens as " + name + " next time)"
	} else if res.Resumed != "" {
		line += ", reopening session " + res.Resumed
	}
	*text = append(*text, line)
	return 0
}

func cmdCurrent(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	cur, err := account.CurrentName(p)
	if err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, map[string]string{"current": cur})
	}
	fmt.Fprintln(stdout, cur)
	return 0
}
