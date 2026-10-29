package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/settings"
	"github.com/zahT3/hotseat/internal/testutil"
)

func settingsFile(t *testing.T, content string) string {
	t.Helper()
	p := twoAccounts(t)
	run(t, "init")
	f := filepath.Join(p.CoreDir(), "settings.json")
	testutil.WriteFile(t, f, content)
	return f
}

func TestStatuslineInstallChainsExisting(t *testing.T) {
	orig := "{\n  \"statusLine\": {\"type\": \"command\", \"command\": \"echo prev\"},\n  \"theme\": \"dark\"\n}\n"
	f := settingsFile(t, orig)
	for i := 0; i < 2; i++ {
		if code, _, stderr := run(t, "statusline", "install"); code != 0 {
			t.Fatalf("install: %s", stderr)
		}
	}
	cmd, _, _ := settings.StatusLineCommand(f)
	if !strings.HasSuffix(cmd, " statusline") {
		t.Errorf("statusLine command = %q", cmd)
	}
	// Running the installed line still shows the old one first.
	restore := setStdin("{}")
	_, out, _ := run(t, "statusline")
	restore()
	if !strings.HasPrefix(out, "prev") {
		t.Errorf("chained output = %q", out)
	}
	if code, _, stderr := run(t, "statusline", "uninstall"); code != 0 {
		t.Fatalf("uninstall: %s", stderr)
	}
	if b, _ := os.ReadFile(f); string(b) != orig {
		t.Errorf("not restored:\n%s", b)
	}
}

func TestHookInstallUninstall(t *testing.T) {
	orig := "{\n  \"hooks\": {\n    \"PreToolUse\": [{\"matcher\": \"Bash\", \"hooks\": [{\"type\": \"command\", \"command\": \"guard\"}]}]\n  }\n}\n"
	f := settingsFile(t, orig)
	run(t, "hook", "install")
	run(t, "hook", "install")
	cmds, _ := settings.HookCommands(f, "StopFailure")
	if len(cmds) != 1 || !strings.HasSuffix(cmds[0], " hook stop-failure") {
		t.Errorf("StopFailure commands = %v", cmds)
	}
	if code, _, stderr := run(t, "hook", "uninstall"); code != 0 {
		t.Fatalf("uninstall: %s", stderr)
	}
	if b, _ := os.ReadFile(f); string(b) != orig {
		t.Errorf("not restored:\n%s", b)
	}
}

const spacedSelf = "'/Applications/My Tools/hotseat'"

func TestStatuslineInstallQuotedSelfPath(t *testing.T) {
	orig := "{\n  \"statusLine\": {\"type\": \"command\", \"command\": \"echo prev\"},\n  \"theme\": \"dark\"\n}\n"
	f := settingsFile(t, orig)
	defer cli.SetSelfCommand(spacedSelf)()
	run(t, "statusline", "install")
	run(t, "statusline", "install")
	p, _ := paths.FromEnv()
	if b, _ := os.ReadFile(p.ChainFile()); strings.Contains(string(b), "hotseat") {
		t.Errorf("chain points at hotseat itself: %s", b)
	}
	if code, _, stderr := run(t, "statusline", "uninstall"); code != 0 {
		t.Fatalf("uninstall: %s", stderr)
	}
	if b, _ := os.ReadFile(f); string(b) != orig {
		t.Errorf("not restored:\n%s", b)
	}
}

func TestHookInstallReplacesOtherHotseatPath(t *testing.T) {
	orig := "{\n  \"hooks\": {\n    \"StopFailure\": [{\"matcher\": \"rate_limit\", \"hooks\": [{\"type\": \"command\", \"command\": \"/old/bin/hotseat hook stop-failure\"}]}]\n  }\n}\n"
	f := settingsFile(t, orig)
	defer cli.SetSelfCommand(spacedSelf)()
	run(t, "hook", "install")
	if cmds, _ := settings.HookCommands(f, "StopFailure"); len(cmds) != 1 || cmds[0] != spacedSelf+" hook stop-failure" {
		t.Errorf("StopFailure commands = %v", cmds)
	}
	run(t, "hook", "uninstall")
	if cmds, _ := settings.HookCommands(f, "StopFailure"); len(cmds) != 0 {
		t.Errorf("uninstall left %v", cmds)
	}
}

func TestStatuslineChainRecursionGuard(t *testing.T) {
	p := statusSetup(t)
	testutil.WriteFile(t, p.ChainFile(), `{"command":"echo prev"}`)
	t.Setenv("HOTSEAT_IN_STATUSLINE", "1")
	_, out := statusline(t, rlInput)
	if strings.HasPrefix(out, "prev") {
		t.Errorf("chained while already inside a chained statusline: %q", out)
	}
}
