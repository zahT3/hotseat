package share_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/share"
	"github.com/zahT3/roster/internal/testutil"
)

// threeAccounts adds claude3 to the two-account fixture.
func threeAccounts(t *testing.T) (fixture, string) {
	t.Helper()
	f := newFixture(t)
	acct3 := filepath.Join(f.p.Home, ".claude3")
	testutil.AddAccountDir(t, f.p, acct3, "c3", "o3", nil)
	f.reg.Accounts = append(f.reg.Accounts, account.Account{Name: "claude3", ConfigDir: acct3, AccountUUID: "c3", OrgUUID: "o3"})
	return f, acct3
}

func kinds(pl share.Plan, item string) map[string]share.ActionKind {
	out := map[string]share.ActionKind{}
	for _, a := range pl.Actions {
		if a.Item == item {
			out[a.Account] = a.Kind
		}
	}
	return out
}

func grepTree(t *testing.T, root, needle string) bool {
	t.Helper()
	found := false
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			if b, _ := os.ReadFile(path); strings.Contains(string(b), needle) {
				found = true
			}
		}
		return nil
	})
	return found
}

func TestPlanAdoptsOnlyOnceAcrossAccounts(t *testing.T) {
	f, acct3 := threeAccounts(t)
	testutil.WriteFile(t, f.acctPath("keybindings.json"), "CLAUDE2 KEYS")
	testutil.WriteFile(t, filepath.Join(acct3, "keybindings.json"), "CLAUDE3 KEYS")
	testutil.WriteFile(t, f.acctPath("themes/a.json"), "a")
	got := kinds(f.plan(t), "keybindings.json")
	if got["claude2"] != share.Adopt || got["claude3"] != share.Conflict {
		t.Errorf("keybindings.json kinds = %v, want claude2 adopt, claude3 conflict", got)
	}
	// claude3 has no themes; once claude2's themes become the shared copy it needs a link.
	if k := kinds(f.plan(t), "themes"); k["claude2"] != share.Adopt || k["claude3"] != share.Link {
		t.Errorf("themes kinds = %v, want claude2 adopt, claude3 link", k)
	}
}

func TestApplyAdoptCollisionLosesNothing(t *testing.T) {
	f, acct3 := threeAccounts(t)
	testutil.WriteFile(t, f.acctPath("keybindings.json"), "CLAUDE2 KEYS")
	testutil.WriteFile(t, filepath.Join(acct3, "keybindings.json"), "CLAUDE3 KEYS")
	if _, err := f.apply(t, share.ApplyOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"CLAUDE2 KEYS", "CLAUDE3 KEYS"} {
		if !grepTree(t, f.p.Home, s) {
			t.Errorf("%q no longer exists anywhere under HOME", s)
		}
	}
}

func TestApplyAdoptRefusesExistingTarget(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.acctPath("keybindings.json"), "ACCT")
	pl := f.plan(t)
	testutil.WriteFile(t, f.core("keybindings.json"), "APPEARED AFTER PLAN")
	if _, err := share.Apply(f.p, f.reg, pl, share.ApplyOptions{Running: noClaude}); err == nil {
		t.Fatal("adopt onto an existing path must fail")
	}
	if b, _ := os.ReadFile(f.core("keybindings.json")); string(b) != "APPEARED AFTER PLAN" {
		t.Errorf("core file overwritten: %q", b)
	}
}

func TestRollbackKeepsPostApplyHistory(t *testing.T) {
	f := richFixture(t)
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := os.OpenFile(f.core("history.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
	h.WriteString(`{"display":"typed after apply","timestamp":99}` + "\n")
	h.Close()
	moved, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(f.core("history.jsonl")); strings.Contains(string(b), "typed after apply") {
		t.Errorf("core history was not restored: %q", b)
	}
	if !grepTree(t, filepath.Join(f.p.BackupsDir(), m.ID), "typed after apply") {
		t.Error("history typed after apply was destroyed by rollback")
	}
	if len(moved) == 0 {
		t.Error("rollback must report where the replaced files went")
	}
}

func TestRollbackMCPKeepsOtherKeys(t *testing.T) {
	f := richFixture(t)
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(f.acct, ".claude.json")
	var doc map[string]any
	b, _ := os.ReadFile(file)
	json.Unmarshal(b, &doc)
	doc["numStartups"] = 99
	doc["projects"] = map[string]any{"/x": map[string]any{"trusted": true}}
	b, _ = json.Marshal(doc)
	os.WriteFile(file, b, 0o600)

	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(file)
	var after map[string]any
	json.Unmarshal(b, &after)
	if after["numStartups"] != float64(99) || after["projects"] == nil {
		t.Errorf("rollback reverted keys other than mcpServers: %v", after)
	}
	servers, _ := after["mcpServers"].(map[string]any)
	if _, ok := servers["a"]; ok || servers["b"] == nil {
		t.Errorf("mcpServers not restored to pre-apply value: %v", servers)
	}
}

func TestPartialCopyIsUndone(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("file-history/s0/v1"), "core")
	testutil.WriteFile(t, f.acctPath("file-history/s1/a"), "a")
	testutil.WriteFile(t, f.acctPath("file-history/s1/z-unreadable"), "z")
	os.Chmod(f.acctPath("file-history/s1/z-unreadable"), 0o000)
	t.Cleanup(func() { os.Chmod(f.acctPath("file-history/s1/z-unreadable"), 0o644) })
	before := snapshot(t, f)

	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err == nil {
		t.Fatal("expected copy failure")
	}
	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	diffSnap(t, before, snapshot(t, f))
}

func TestRollbackRetryAfterPartialFailure(t *testing.T) {
	f := richFixture(t)
	before := snapshot(t, f)
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	// Steps get half undone: links removed and account copies restored, then
	// moving the copies roster added to core aside fails.
	aside := filepath.Join(f.p.BackupsDir(), m.ID, "rollback-removed")
	mkdir(t, aside)
	os.Chmod(aside, 0o555)
	t.Cleanup(func() { os.Chmod(aside, 0o755) })
	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err == nil {
		t.Fatal("expected rollback to fail while rollback-removed is read-only")
	}
	os.Chmod(aside, 0o755)
	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err != nil {
		t.Fatalf("retry after fixing the cause: %v", err)
	}
	diffSnap(t, before, snapshot(t, f))
	again, _ := share.LoadManifest(f.p, m.ID)
	if !again.RolledBack {
		t.Error("manifest not marked rolled back after a successful retry")
	}
}

func TestRollbackRefusesWhenClaudeRunning(t *testing.T) {
	f := richFixture(t)
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, f)
	_, err = share.Rollback(f.p, m.ID, share.RollbackOptions{Running: func() ([]int, error) { return []int{7}, nil }})
	if !errors.Is(err, share.ErrClaudeRunning) {
		t.Fatalf("err = %v, want ErrClaudeRunning", err)
	}
	diffSnap(t, after, snapshot(t, f))
}

func TestPlanSkipsAccountWithMissingDir(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("projects"))
	f.reg.Accounts = append(f.reg.Accounts, account.Account{Name: "gone", ConfigDir: filepath.Join(f.p.Home, ".claude-gone"), AccountUUID: "g"})
	pl := f.plan(t)
	for _, a := range pl.Actions {
		if a.Account == "gone" {
			t.Errorf("action planned for missing dir: %+v", a)
		}
	}
	if len(pl.Skipped) != 1 || !strings.Contains(pl.Skipped[0], "gone") {
		t.Errorf("Skipped = %v", pl.Skipped)
	}
	if _, err := share.Apply(f.p, f.reg, pl, share.ApplyOptions{Running: noClaude}); err != nil {
		t.Errorf("apply should succeed for the remaining accounts: %v", err)
	}
}

func TestSyncMCPConvergesWithHTMLChars(t *testing.T) {
	f := newFixture(t)
	// Written like Claude Code does: JSON.stringify does not escape & < >.
	testutil.WriteFile(t, f.p.DefaultClaudeJSON(),
		`{"oauthAccount":{"accountUuid":"a1","organizationUuid":"o1"},"mcpServers":{"web":{"type":"http","url":"https://x.test/?a=1&b=<2>"}}}`)
	testutil.AddAccountDir(t, f.p, f.acct, "b2", "o2", map[string]any{"other": map[string]any{"command": "y"}})
	if _, err := f.apply(t, share.ApplyOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	for _, a := range f.plan(t).Actions {
		if a.Kind == share.SyncMCP {
			t.Errorf("sync-mcp still planned after apply: %+v", a)
		}
	}
	b, _ := os.ReadFile(filepath.Join(f.acct, ".claude.json"))
	if !strings.Contains(string(b), "a=1&b=<2>") {
		t.Errorf("value was HTML-escaped on write: %s", b)
	}
}
