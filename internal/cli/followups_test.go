package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/testutil"
)

func TestAddRejectsCoreDir(t *testing.T) {
	p := addSetup(t)
	fakeClaude(t, `{"oauthAccount":{"accountUuid":"n1","organizationUuid":"o9"}}`)
	for _, dir := range []string{p.CoreDir(), filepath.Join(p.Home, ".claude2")} {
		code, _, stderr := run(t, "add", "x", "--dir", dir)
		if code != 1 || !strings.Contains(stderr, "already") {
			t.Errorf("--dir %s: exit %d, %q", dir, code, stderr)
		}
	}
}

func TestApplyNothingToDoMessage(t *testing.T) {
	twoAccounts(t)
	run(t, "init")
	code, out, stderr := run(t, "share", "apply", "--force")
	if code != 0 || !strings.Contains(out, "nothing to do") {
		t.Errorf("exit %d out %q err %q", code, out, stderr)
	}
}

func TestEveryCommandSupportsJSON(t *testing.T) {
	p := addSetup(t)
	testutil.WriteFile(t, p.Zshrc(), "export A=1\n")
	testutil.WriteFile(t, filepath.Join(p.Home, ".claude2", "history.jsonl"), `{"timestamp":5}`+"\n")
	fakeClaude(t, `{"oauthAccount":{"accountUuid":"n1","organizationUuid":"o9"}}`)

	_, applyOut, _ := run(t, "share", "apply", "--force", "--json")
	var m struct{ ID string }
	if err := json.Unmarshal([]byte(applyOut), &m); err != nil || m.ID == "" {
		t.Fatalf("apply --json = %q", applyOut)
	}
	cmds := [][]string{
		{"version", "--json"},
		{"current", "--json"},
		{"use", "claude2", "--json"},
		{"share", "set", "scheduled-tasks", "local", "--json"},
		{"share", "backups", "--json"},
		{"share", "rollback", m.ID, "--force", "--json"},
		{"shim", "install", "--json"},
		{"shim", "uninstall", "--json"},
		{"add", "work", "--json"},
	}
	for _, c := range cmds {
		code, out, stderr := run(t, c...)
		var v any
		if code != 0 || json.Unmarshal([]byte(out), &v) != nil {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", c, code, out, stderr)
		}
	}
	_ = os.Remove
}
