package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/settings"
	"github.com/zahT3/hotseat/internal/testutil"
)

type setupOut struct {
	Version      string `json:"version"`
	Accounts     int    `json:"accounts"`
	LoginsFound  int    `json:"loginsFound"`
	ShareActions int    `json:"shareActions"`
	Shim         bool   `json:"shim"`
	Statusline   bool   `json:"statusline"`
	Hook         bool   `json:"hook"`
	Desktop      string `json:"desktop"`
}

func setupStatus(t *testing.T) (int, setupOut) {
	t.Helper()
	code, out, stderr := run(t, "setup", "status", "--json")
	var v setupOut
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("exit %d, bad json %q (stderr %q): %v", code, out, stderr, err)
	}
	return code, v
}

func TestSetupStatusBlankMachine(t *testing.T) {
	testutil.FakeHome(t)
	code, v := setupStatus(t)
	if code != 0 || v.Accounts != 0 || v.LoginsFound != 0 || v.Shim || v.Statusline || v.Hook || v.Desktop != "absent" || v.Version == "" {
		t.Errorf("exit %d, %+v", code, v)
	}
}

func TestSetupStatusAfterSteps(t *testing.T) {
	p := twoAccounts(t)
	defer cli.SetSelfCommand("/usr/local/bin/hotseat")()
	for _, args := range [][]string{{"init"}, {"shim", "install"}, {"statusline", "install"}, {"hook", "install"}} {
		if code, _, stderr := run(t, args...); code != 0 {
			t.Fatalf("%v: %s", args, stderr)
		}
	}
	testutil.WriteFile(t, filepath.Join(p.DesktopDataDir(), "config.json"), `{"lastKnownAccountUuid":"a1"}`)
	code, v := setupStatus(t)
	if code != 0 || v.Accounts != 2 || v.LoginsFound != 2 || !v.Shim || !v.Statusline || !v.Hook || v.Desktop != "available" {
		t.Errorf("exit %d, %+v", code, v)
	}
}

// A fresh Mac may have no ~/.claude/settings.json yet.
func TestIntegrationsInstallWithoutSettingsFile(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	defer cli.SetSelfCommand("/usr/local/bin/hotseat")()
	for _, args := range [][]string{{"statusline", "install"}, {"hook", "install"}} {
		if code, _, stderr := run(t, args...); code != 0 {
			t.Fatalf("%v: %s", args, stderr)
		}
	}
	if cmd, _, _ := settings.StatusLineCommand(filepath.Join(p.CoreDir(), "settings.json")); cmd != "/usr/local/bin/hotseat statusline" {
		t.Errorf("statusLine = %q", cmd)
	}
}
