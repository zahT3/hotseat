package cli_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func twoAccounts(t *testing.T) paths.Paths {
	t.Helper()
	p := testutil.FakeHome(t)
	testutil.AddAccountDir(t, p, p.CoreDir(), "a1", "o1", nil)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claude2"), "b2", "o2", nil)
	return p
}

func TestInitIsIdempotentAndSetsCurrent(t *testing.T) {
	p := twoAccounts(t)
	if code, _, stderr := run(t, "init"); code != 0 {
		t.Fatalf("init exit %d: %s", code, stderr)
	}
	if cur, _ := account.CurrentName(p); cur != "claude" {
		t.Errorf("current = %q, want claude", cur)
	}
	if code, _, stderr := run(t, "use", "claude2"); code != 0 {
		t.Fatalf("use exit %d: %s", code, stderr)
	}
	if code, _, stderr := run(t, "init"); code != 0 {
		t.Fatalf("second init exit %d: %s", code, stderr)
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Accounts) != 2 {
		t.Errorf("registry has %d accounts, want 2", len(reg.Accounts))
	}
	if cur, _ := account.CurrentName(p); cur != "claude2" {
		t.Errorf("current = %q, want claude2 kept", cur)
	}
}

func TestInitKeepsRenamedAccount(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	reg, _ := account.LoadRegistry(p)
	reg.Accounts[1].Name = "work"
	if err := account.SaveRegistry(p, reg); err != nil {
		t.Fatal(err)
	}
	run(t, "init")
	reg, _ = account.LoadRegistry(p)
	if _, ok := reg.Find("work"); !ok {
		t.Errorf("rename lost after re-init: %+v", reg.Accounts)
	}
	if _, ok := reg.Find("claude2"); ok {
		t.Errorf("duplicate entry re-added: %+v", reg.Accounts)
	}
}

func TestUseUnknownAccountFails(t *testing.T) {
	twoAccounts(t)
	run(t, "init")
	code, _, stderr := run(t, "use", "nope")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown account "nope"`) {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestAccountsListsCurrentAndJSON(t *testing.T) {
	twoAccounts(t)
	run(t, "init")
	_, out, _ := run(t, "accounts")
	if !strings.Contains(out, "* claude ") || !strings.Contains(out, "~/.claude2") {
		t.Errorf("accounts output:\n%s", out)
	}
	_, out, _ = run(t, "accounts", "--json")
	var doc struct {
		Current  string            `json:"current"`
		Accounts []account.Account `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("bad json %q: %v", out, err)
	}
	if doc.Current != "claude" || len(doc.Accounts) != 2 {
		t.Errorf("json = %+v", doc)
	}
	_, out, _ = run(t, "current")
	if strings.TrimSpace(out) != "claude" {
		t.Errorf("current = %q", out)
	}
}
