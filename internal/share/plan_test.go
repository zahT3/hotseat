package share_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/share"
	"github.com/zahT3/hotseat/internal/testutil"
)

type fixture struct {
	p    paths.Paths
	reg  account.Registry
	acct string // claude2 config dir
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	p := testutil.FakeHome(t)
	acct := filepath.Join(p.Home, ".claude2")
	testutil.AddAccountDir(t, p, p.CoreDir(), "a1", "o1", nil)
	testutil.AddAccountDir(t, p, acct, "b2", "o2", nil)
	reg := account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true, AccountUUID: "a1", OrgUUID: "o1"},
		{Name: "claude2", ConfigDir: acct, AccountUUID: "b2", OrgUUID: "o2"},
	}}
	return fixture{p: p, reg: reg, acct: acct}
}

func (f fixture) plan(t *testing.T) share.Plan {
	t.Helper()
	c, err := share.LoadClassifier(f.p)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := share.MakePlan(f.p, f.reg, c)
	if err != nil {
		t.Fatal(err)
	}
	return pl
}

func (f fixture) core(item string) string     { return filepath.Join(f.p.CoreDir(), item) }
func (f fixture) acctPath(item string) string { return filepath.Join(f.acct, item) }

func findAction(pl share.Plan, acct, item string) (share.Action, bool) {
	for _, a := range pl.Actions {
		if a.Account == acct && a.Item == item {
			return a, true
		}
	}
	return share.Action{}, false
}

func expectKind(t *testing.T, pl share.Plan, item string, want share.ActionKind) {
	t.Helper()
	a, ok := findAction(pl, "claude2", item)
	if want == "" {
		if ok {
			t.Errorf("%s: unexpected action %+v", item, a)
		}
		return
	}
	if !ok || a.Kind != want {
		t.Errorf("%s: got %+v (found=%v), want %s", item, a, ok, want)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestPlanBothMissing_NoAction(t *testing.T) {
	f := newFixture(t)
	expectKind(t, f.plan(t), "themes", "")
}

func TestPlanCoreMissingAcctReal_Adopt(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.acctPath("keybindings.json"), "{}")
	expectKind(t, f.plan(t), "keybindings.json", share.Adopt)
}

func TestPlanCoreMissingAcctSymlink_Conflict(t *testing.T) {
	f := newFixture(t)
	symlink(t, "/nowhere", f.acctPath("themes"))
	expectKind(t, f.plan(t), "themes", share.Conflict)
}

func TestPlanCoreExistsAcctMissing_Link(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("projects"))
	a, _ := findAction(f.plan(t), "claude2", "projects")
	if a.Kind != share.Link || a.Path != f.acctPath("projects") || a.Target != f.core("projects") {
		t.Errorf("got %+v", a)
	}
}

func TestPlanAcctSymlinkToCore_NoAction(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("projects"))
	symlink(t, f.core("projects"), f.acctPath("projects"))
	expectKind(t, f.plan(t), "projects", "")
}

func TestPlanAcctSymlinkElsewhere_Conflict(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("projects"))
	mkdir(t, f.core("plans"))
	elsewhere := filepath.Join(f.p.Home, "elsewhere")
	mkdir(t, elsewhere)
	symlink(t, elsewhere, f.acctPath("projects"))
	symlink(t, filepath.Join(f.p.Home, "gone"), f.acctPath("plans"))
	pl := f.plan(t)
	expectKind(t, pl, "projects", share.Conflict)
	expectKind(t, pl, "plans", share.Conflict)
}

func TestPlanHistory_MergeJSONL(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("history.jsonl"), `{"timestamp":1}`+"\n")
	testutil.WriteFile(t, f.acctPath("history.jsonl"), `{"timestamp":2}`+"\n")
	expectKind(t, f.plan(t), "history.jsonl", share.MergeJSONL)
}

func TestPlanIdenticalFile_ReplaceWithLink(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("settings.json"), `{"a":1}`)
	testutil.WriteFile(t, f.acctPath("settings.json"), `{"a":1}`)
	expectKind(t, f.plan(t), "settings.json", share.ReplaceWithLink)
}

func TestPlanDifferentFile_Conflict(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("CLAUDE.md"), "core")
	testutil.WriteFile(t, f.acctPath("CLAUDE.md"), "acct")
	expectKind(t, f.plan(t), "CLAUDE.md", share.Conflict)
}

func TestPlanDirs_MergeDir(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("skills/a/SKILL.md"), "a")
	testutil.WriteFile(t, f.acctPath("skills/b/SKILL.md"), "b")
	expectKind(t, f.plan(t), "skills", share.MergeDir)
}

func TestPlanTypeMismatch_Conflict(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("tasks"))
	testutil.WriteFile(t, f.acctPath("tasks"), "file")
	expectKind(t, f.plan(t), "tasks", share.Conflict)
}

func TestPlanTreatsEquivalentSymlinkAsLinked(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("projects"))
	hop := filepath.Join(f.p.Home, "hop")
	symlink(t, f.core("projects"), hop)
	symlink(t, hop, f.acctPath("projects"))
	expectKind(t, f.plan(t), "projects", "")
}

func TestPlanSkipsDefaultAccount(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("projects"))
	for _, a := range f.plan(t).Actions {
		if a.Account == "claude" {
			t.Errorf("default account got action %+v", a)
		}
	}
}

func TestPlanUsesOverrides(t *testing.T) {
	f := newFixture(t)
	mkdir(t, f.core("scheduled-tasks"))
	if err := share.SetOverride(f.p, "scheduled-tasks", share.Shared); err != nil {
		t.Fatal(err)
	}
	expectKind(t, f.plan(t), "scheduled-tasks", share.Link)
}

func TestPlanSyncMCPOnlyWhenDifferent(t *testing.T) {
	f := newFixture(t)
	for _, a := range f.plan(t).Actions {
		if a.Kind == share.SyncMCP {
			t.Fatalf("no MCP servers anywhere, got %+v", a)
		}
	}
	testutil.AddAccountDir(t, f.p, f.acct, "b2", "o2", map[string]any{"b": map[string]any{"command": "y"}})
	var got *share.Action
	for _, a := range f.plan(t).Actions {
		if a.Kind == share.SyncMCP {
			got = &a
		}
	}
	if got == nil || !strings.Contains(got.Detail, "b") {
		t.Errorf("want sync-mcp mentioning b, got %+v", got)
	}
}
