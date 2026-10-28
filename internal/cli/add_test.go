package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/paths"
	"github.com/zahT3/roster/internal/testutil"
)

// fakeClaude puts a `claude` first on PATH that, on `auth login`, writes
// $FAKE_LOGIN_JSON into the new config dir like a completed login would.
func fakeClaude(t *testing.T, loginJSON string) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "auth" ] && [ "$2" = "login" ] && [ -n "$FAKE_LOGIN_JSON" ]; then
  printf '%s' "$FAKE_LOGIN_JSON" > "$CLAUDE_CONFIG_DIR/.claude.json"
fi
`
	testutil.WriteFile(t, filepath.Join(bin, "claude"), script)
	if err := os.Chmod(filepath.Join(bin, "claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("FAKE_LOGIN_JSON", loginJSON)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
}

func addSetup(t *testing.T) paths.Paths {
	t.Helper()
	p := twoAccounts(t)
	if err := os.MkdirAll(filepath.Join(p.CoreDir(), "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, filepath.Join(p.CoreDir(), "history.jsonl"), "{}\n")
	if code, _, stderr := run(t, "init"); code != 0 {
		t.Fatalf("init: %s", stderr)
	}
	return p
}

func TestAddRegistersAndLinks(t *testing.T) {
	p := addSetup(t)
	fakeClaude(t, `{"oauthAccount":{"accountUuid":"n1","organizationUuid":"o9"}}`)

	code, out, stderr := run(t, "add", "work")
	if code != 0 {
		t.Fatalf("add exit %d: %s", code, stderr)
	}
	reg, _ := account.LoadRegistry(p)
	a, ok := reg.Find("work")
	if !ok || a.AccountUUID != "n1" || a.ConfigDir != filepath.Join(p.Home, ".claude-work") {
		t.Fatalf("registry = %+v", reg.Accounts)
	}
	for _, item := range []string{"projects", "history.jsonl"} {
		link := filepath.Join(a.ConfigDir, item)
		target, err := os.Readlink(link)
		if err != nil || target != filepath.Join(p.CoreDir(), item) {
			t.Errorf("%s -> %q, %v", item, target, err)
		}
	}
	info, _ := os.Stat(a.ConfigDir)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("dir perm = %o", info.Mode().Perm())
	}
	if !strings.Contains(out, "roster share apply") {
		t.Errorf("missing MCP hint in output:\n%s", out)
	}
}

func TestAddDuplicateAccount(t *testing.T) {
	addSetup(t)
	fakeClaude(t, `{"oauthAccount":{"accountUuid":"b2","organizationUuid":"o2"}}`)
	code, _, stderr := run(t, "add", "again")
	if code != 1 || !strings.Contains(stderr, `already registered as "claude2"`) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestAddLoginNotCompleted(t *testing.T) {
	p := addSetup(t)
	fakeClaude(t, "")
	code, _, stderr := run(t, "add", "work")
	if code != 1 || !strings.Contains(stderr, "login did not complete") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	reg, _ := account.LoadRegistry(p)
	if _, ok := reg.Find("work"); ok {
		t.Error("account registered without a login")
	}
}

func TestAddRejectsExistingNameOrDir(t *testing.T) {
	p := addSetup(t)
	fakeClaude(t, `{"oauthAccount":{"accountUuid":"n1","organizationUuid":"o9"}}`)
	if code, _, stderr := run(t, "add", "claude2"); code != 1 || !strings.Contains(stderr, `account "claude2" already exists`) {
		t.Errorf("existing name: exit %d, %q", code, stderr)
	}
	testutil.WriteFile(t, filepath.Join(p.Home, ".claude-busy", "x"), "x")
	if code, _, stderr := run(t, "add", "busy"); code != 1 || !strings.Contains(stderr, "is not empty") {
		t.Errorf("non-empty dir: exit %d, %q", code, stderr)
	}
	if code, _, stderr := run(t, "add", "../evil"); code != 1 || !strings.Contains(stderr, "invalid account name") {
		t.Errorf("bad name: exit %d, %q", code, stderr)
	}
}
