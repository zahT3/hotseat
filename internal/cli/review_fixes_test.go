package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/settings"
	"github.com/zahT3/hotseat/internal/testutil"
)

// Files that differ between accounts are conflicts apply leaves alone; they
// must not keep the Share step open forever.
func TestSetupStatusCountsConflictsApart(t *testing.T) {
	p := twoAccounts(t)
	testutil.WriteFile(t, filepath.Join(p.CoreDir(), "settings.json"), `{"model":"opus"}`)
	testutil.WriteFile(t, filepath.Join(p.Home, ".claude2", "settings.json"), `{"model":"sonnet"}`)
	run(t, "init")
	_, plan, _ := run(t, "share", "plan", "--json")
	var pl struct {
		Actions []struct{ Kind string } `json:"actions"`
	}
	json.Unmarshal([]byte(plan), &pl)
	conflicts := 0
	for _, a := range pl.Actions {
		if a.Kind == "conflict" {
			conflicts++
		}
	}
	if conflicts == 0 {
		t.Fatalf("setup: no conflict in plan %s", plan)
	}
	_, out, _ := run(t, "setup", "status", "--json")
	var v struct {
		ShareActions   int `json:"shareActions"`
		ShareConflicts int `json:"shareConflicts"`
	}
	json.Unmarshal([]byte(out), &v)
	if v.ShareConflicts != conflicts || v.ShareActions != len(pl.Actions)-conflicts {
		t.Errorf("status %s, plan has %d actions, %d conflicts", out, len(pl.Actions), conflicts)
	}
}

func TestUninstallWithoutSettingsFile(t *testing.T) {
	twoAccounts(t)
	run(t, "init")
	run(t, "shim", "install")
	code, out, stderr := run(t, "uninstall", "--yes")
	if code != 0 || strings.Contains(out, "failed") {
		t.Errorf("exit %d out %q err %q", code, out, stderr)
	}
}

func TestUninstallLeavesDeletedStatusLine(t *testing.T) {
	f := installAll(t)
	testutil.WriteFile(t, f, "{}\n")
	code, out, _ := run(t, "uninstall", "--yes")
	if code != 0 || !strings.Contains(out, "left as is") {
		t.Errorf("exit %d out %q", code, out)
	}
	if cmd, raw, _ := settings.StatusLineCommand(f); raw != nil {
		t.Errorf("statusLine brought back: %q", cmd)
	}
}

// Homebrew's Cellar path changes on every upgrade (and the old one is
// cleaned up); settings.json and the shim must point at the opt link.
func TestStableExecutablePath(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/Cellar/hotseat/0.1.0/bin/hotseat": "/opt/homebrew/opt/hotseat/bin/hotseat",
		"/usr/local/Cellar/hotseat/0.2.0_1/bin/hotseat":  "/usr/local/opt/hotseat/bin/hotseat",
		"/Users/u/.local/bin/hotseat":                    "/Users/u/.local/bin/hotseat",
	}
	for in, want := range cases {
		if got := cli.StablePath(in); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}
