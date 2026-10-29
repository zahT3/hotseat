package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/paths"
)

type pressRunner struct {
	press  string
	opened []string
}

func (r *pressRunner) Dialog(string, []string, string) (string, error) { return r.press, nil }
func (r *pressRunner) Notify(string, string) error                     { return nil }
func (r *pressRunner) OpenTerminal(c string) error                     { r.opened = append(r.opened, c); return nil }
func (r *pressRunner) SwitchDesktop(paths.Paths, account.Registry, string, string) error {
	return nil
}

func TestHandoffCommand(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	run(t, "use", "claude2")
	r := &pressRunner{press: "Continue as claude"}
	defer cli.SetHandoffRunner(r)()
	code, _, stderr := run(t, "handoff", "--from", "claude2", "--to", "claude", "--session", "s1",
		"--cwd", "/tmp/x", "--tokens", "180002", "--resets", "2000000000", "--message", "limit")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if cur, _ := account.CurrentName(p); cur != "claude" || len(r.opened) != 1 || !strings.Contains(r.opened[0], "--resume s1") {
		t.Errorf("current %q opened %v", cur, r.opened)
	}
}

func TestHandoffCommandLogsErrors(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	defer cli.SetHandoffRunner(&pressRunner{press: "Continue as nobody"})()
	code, _, _ := run(t, "handoff", "--from", "claude2", "--to", "nobody", "--session", "s1")
	b, _ := os.ReadFile(p.StateDir + "/handoff.log")
	if code != 1 || !strings.Contains(string(b), "nobody") {
		t.Errorf("exit %d log %q", code, b)
	}
}
