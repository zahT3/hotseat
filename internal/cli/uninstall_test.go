package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/settings"
)

func installAll(t *testing.T) string {
	t.Helper()
	f := settingsFile(t, "{\n  \"statusLine\": {\"type\": \"command\", \"command\": \"echo mine\"}\n}\n")
	t.Cleanup(cli.SetSelfCommand("/usr/local/bin/hotseat"))
	for _, args := range [][]string{{"shim", "install"}, {"statusline", "install"}, {"hook", "install"}} {
		if code, _, stderr := run(t, args...); code != 0 {
			t.Fatalf("%v: %s", args, stderr)
		}
	}
	return f
}

func zshrc(t *testing.T, f string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(f)), ".zshrc"))
	return string(b)
}

func TestUninstallNeedsYes(t *testing.T) {
	f := installAll(t)
	code, out, _ := run(t, "uninstall")
	if code != 1 || !strings.Contains(out, "share rollback") || !strings.Contains(out, "--yes") {
		t.Errorf("exit %d out %q", code, out)
	}
	if !strings.Contains(zshrc(t, f), ">>> hotseat >>>") {
		t.Error("zshrc changed without --yes")
	}
}

func TestUninstallRemovesIntegrations(t *testing.T) {
	f := installAll(t)
	if code, out, stderr := run(t, "uninstall", "--yes"); code != 0 {
		t.Fatalf("exit %d out %q err %q", code, out, stderr)
	}
	if strings.Contains(zshrc(t, f), ">>> hotseat >>>") {
		t.Error("shim block left in zshrc")
	}
	if cmd, _, _ := settings.StatusLineCommand(f); cmd != "echo mine" {
		t.Errorf("statusLine = %q, want the user's own back", cmd)
	}
	if cmds, _ := settings.HookCommands(f, "StopFailure"); len(cmds) != 0 {
		t.Errorf("hooks left: %v", cmds)
	}
}

func TestUninstallKeepsChangedStatusLine(t *testing.T) {
	f := installAll(t)
	if _, err := settings.SetStatusLine(f, "echo changed", filepath.Join(t.TempDir(), "b.json")); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run(t, "uninstall", "--yes")
	if code != 0 || !strings.Contains(out, "left as is") {
		t.Errorf("exit %d out %q", code, out)
	}
	if cmd, _, _ := settings.StatusLineCommand(f); cmd != "echo changed" {
		t.Errorf("statusLine = %q", cmd)
	}
	if cmds, _ := settings.HookCommands(f, "StopFailure"); len(cmds) != 0 {
		t.Errorf("hook not removed after the statusLine step: %v", cmds)
	}
}
