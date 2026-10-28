package doctor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/doctor"
	"github.com/zahT3/roster/internal/paths"
	"github.com/zahT3/roster/internal/testutil"
)

type fx struct {
	p    paths.Paths
	reg  account.Registry
	acct string
}

// cleanSetup: two logged-in accounts, projects shared via symlink, nothing
// unknown, shim first on PATH.
func cleanSetup(t *testing.T) (fx, []string) {
	t.Helper()
	p := testutil.FakeHome(t)
	acct := filepath.Join(p.Home, ".claude2")
	testutil.AddAccountDir(t, p, p.CoreDir(), "a1", "o1", nil)
	testutil.AddAccountDir(t, p, acct, "b2", "o2", nil)
	if err := os.MkdirAll(filepath.Join(p.CoreDir(), "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(p.CoreDir(), "projects"), filepath.Join(acct, "projects")); err != nil {
		t.Fatal(err)
	}
	reg := account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true, AccountUUID: "a1", OrgUUID: "o1"},
		{Name: "claude2", ConfigDir: acct, AccountUUID: "b2", OrgUUID: "o2"},
	}}
	roster := filepath.Join(p.Home, "roster-bin")
	testutil.WriteFile(t, roster, "#!/bin/sh\n")
	os.Chmod(roster, 0o755)
	os.MkdirAll(p.ShimDir(), 0o755)
	if err := os.Symlink(roster, filepath.Join(p.ShimDir(), "claude")); err != nil {
		t.Fatal(err)
	}
	env := []string{"PATH=" + p.ShimDir() + ":/usr/bin"}
	return fx{p: p, reg: reg, acct: acct}, env
}

func check(t *testing.T, f fx, env []string) []doctor.Finding {
	t.Helper()
	got, err := doctor.Check(f.p, f.reg, env)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func has(findings []doctor.Finding, code, level string) bool {
	for _, x := range findings {
		if x.Code == code && x.Level == level {
			return true
		}
	}
	return false
}

func TestDoctorCleanSetupHasNoErrors(t *testing.T) {
	f, env := cleanSetup(t)
	for _, x := range check(t, f, env) {
		if x.Level == "error" || x.Level == "warn" {
			t.Errorf("unexpected finding %+v", x)
		}
	}
}

func TestDoctorAccountDirMissing(t *testing.T) {
	f, env := cleanSetup(t)
	f.reg.Accounts = append(f.reg.Accounts, account.Account{Name: "gone", ConfigDir: filepath.Join(f.p.Home, ".claude-gone"), AccountUUID: "g"})
	got := check(t, f, env)
	if !has(got, "account-dir-missing", "error") {
		t.Errorf("findings = %+v", got)
	}
	for _, x := range got {
		if x.Account == "gone" && x.Code != "account-dir-missing" {
			t.Errorf("missing dir should suppress other findings, got %+v", x)
		}
	}
}

func TestDoctorIdentityMismatch(t *testing.T) {
	f, env := cleanSetup(t)
	testutil.AddAccountDir(t, f.p, f.acct, "someone-else", "o9", nil)
	if !has(check(t, f, env), "identity-mismatch", "error") {
		t.Error("identity-mismatch not reported")
	}
}

func TestDoctorIdentityMissing(t *testing.T) {
	f, env := cleanSetup(t)
	testutil.AddAccountDir(t, f.p, f.acct, "", "", nil)
	if !has(check(t, f, env), "identity-missing", "warn") {
		t.Error("identity-missing not reported")
	}
}

func TestDoctorSharedMissing(t *testing.T) {
	f, env := cleanSetup(t)
	os.MkdirAll(filepath.Join(f.p.CoreDir(), "plans"), 0o755)
	if !has(check(t, f, env), "shared-missing", "warn") {
		t.Error("shared-missing not reported")
	}
}

func TestDoctorSharedDiverged(t *testing.T) {
	f, env := cleanSetup(t)
	testutil.WriteFile(t, filepath.Join(f.p.CoreDir(), "history.jsonl"), "{}\n")
	testutil.WriteFile(t, filepath.Join(f.acct, "history.jsonl"), "{}\n")
	if !has(check(t, f, env), "shared-diverged", "error") {
		t.Error("shared-diverged not reported")
	}
}

func TestDoctorSharedConflict(t *testing.T) {
	f, env := cleanSetup(t)
	testutil.WriteFile(t, filepath.Join(f.p.CoreDir(), "CLAUDE.md"), "a")
	testutil.WriteFile(t, filepath.Join(f.acct, "CLAUDE.md"), "b")
	if !has(check(t, f, env), "shared-conflict", "error") {
		t.Error("shared-conflict not reported")
	}
}

func TestDoctorMCPDiverged(t *testing.T) {
	f, env := cleanSetup(t)
	testutil.AddAccountDir(t, f.p, f.acct, "b2", "o2", map[string]any{"x": map[string]any{}})
	if !has(check(t, f, env), "mcp-diverged", "warn") {
		t.Error("mcp-diverged not reported")
	}
}

func TestDoctorUnknownEntry(t *testing.T) {
	f, env := cleanSetup(t)
	testutil.WriteFile(t, filepath.Join(f.acct, "scheduled-tasks", "x.json"), "{}")
	got := check(t, f, env)
	found := false
	for _, x := range got {
		if x.Code == "unknown-entry" && x.Level == "warn" && strings.Contains(x.Message, "roster share set scheduled-tasks shared|local") {
			found = true
		}
	}
	if !found {
		t.Errorf("findings = %+v", got)
	}
}

func TestDoctorPendingEntry(t *testing.T) {
	f, env := cleanSetup(t)
	os.MkdirAll(filepath.Join(f.acct, "plugins"), 0o755)
	if !has(check(t, f, env), "pending-entry", "info") {
		t.Error("pending-entry not reported")
	}
}

func TestDoctorOAuthTokenValueNotPrinted(t *testing.T) {
	f, env := cleanSetup(t)
	env = append(env, "CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-SECRET")
	testutil.WriteFile(t, filepath.Join(f.p.CoreDir(), "settings.json"), `{"env":{"CLAUDE_CODE_OAUTH_TOKEN":"sk-ant-oat01-SECRET"}}`)
	os.Symlink(filepath.Join(f.p.CoreDir(), "settings.json"), filepath.Join(f.acct, "settings.json"))
	got := check(t, f, env)
	n := 0
	for _, x := range got {
		if x.Code == "oauth-token-env" && x.Level == "warn" {
			n++
		}
		if strings.Contains(x.Message+x.Path+x.Account, "SECRET") {
			t.Errorf("token value leaked: %+v", x)
		}
	}
	if n != 2 {
		t.Errorf("want 2 oauth-token-env findings (env + settings), got %d: %+v", n, got)
	}
}

func TestDoctorShimInactive(t *testing.T) {
	f, _ := cleanSetup(t)
	other := filepath.Join(f.p.Home, "otherbin")
	testutil.WriteFile(t, filepath.Join(other, "claude"), "#!/bin/sh\n")
	os.Chmod(filepath.Join(other, "claude"), 0o755)
	env := []string{"PATH=" + other + ":" + f.p.ShimDir()}
	if !has(check(t, f, env), "shim-inactive", "info") {
		t.Error("shim-inactive not reported")
	}
}
