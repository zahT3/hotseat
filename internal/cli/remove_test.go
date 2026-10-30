package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/testutil"
	"github.com/zahT3/hotseat/internal/usage"
)

func stubLogout(t *testing.T) *[]string {
	t.Helper()
	var dirs []string
	t.Cleanup(cli.SetAuthLogout(func(dir string) error { dirs = append(dirs, dir); return nil }))
	return &dirs
}

func TestRemoveMovesDirToBackupAndUnregisters(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	logouts := stubLogout(t)
	usage.Save(p, usage.Record{Account: "claude2"})
	dir := filepath.Join(p.Home, ".claude2")
	if code, _, stderr := run(t, "remove", "claude2", "--yes"); code != 0 {
		t.Fatalf("remove: %s", stderr)
	}
	reg, _ := account.LoadRegistry(p)
	if _, ok := reg.Find("claude2"); ok {
		t.Error("still registered")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("config dir left in place")
	}
	if found, _ := filepath.Glob(filepath.Join(p.BackupsDir(), "remove-claude2-*", "config", ".claude.json")); len(found) != 1 {
		t.Errorf("config dir not in backup: %v", found)
	}
	if found, _ := filepath.Glob(filepath.Join(p.BackupsDir(), "remove-claude2-*", "usage.json")); len(found) != 1 {
		t.Errorf("usage record not in backup: %v", found)
	}
	if len(*logouts) != 1 || (*logouts)[0] != dir {
		t.Errorf("logout calls %v", *logouts)
	}
}

func TestRemoveNeedsYes(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	stubLogout(t)
	code, out, _ := run(t, "remove", "claude2")
	if code != 1 || !strings.Contains(out, "--yes") {
		t.Errorf("exit %d out %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(p.Home, ".claude2")); err != nil {
		t.Error("dir moved without --yes")
	}
}

func TestRemoveRefusesCurrentDefaultAndDesktop(t *testing.T) {
	p := desktopSetup(t)
	stubLogout(t)
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	run(t, "use", "claude2", "--cli-only")
	if code, _, stderr := run(t, "remove", "claude2", "--yes"); code != 1 || !strings.Contains(stderr, "current") {
		t.Errorf("removed the current account: %d %q", code, stderr)
	}
	run(t, "use", "claude", "--cli-only")
	if code, _, stderr := run(t, "remove", "claude", "--yes"); code != 1 || !strings.Contains(stderr, "default") {
		t.Errorf("removed the default account: %d %q", code, stderr)
	}
	s, _ := desktop.LoadState(p)
	s.Active = "claude2"
	desktop.SaveState(p, s)
	if code, _, stderr := run(t, "remove", "claude2", "--yes"); code != 1 || !strings.Contains(stderr, "Desktop") {
		t.Errorf("removed the Desktop account: %d %q", code, stderr)
	}
}

func TestRemoveMovesParkedDesktopProfile(t *testing.T) {
	p := desktopSetup(t)
	stubLogout(t)
	writeDesktopDir(t, p.DesktopProfileDir("claude2"), "b2")
	s, _ := desktop.LoadState(p)
	s.Profiles["claude2"] = "b2"
	desktop.SaveState(p, s)
	if code, _, stderr := run(t, "remove", "claude2", "--yes"); code != 0 {
		t.Fatalf("remove: %s", stderr)
	}
	if _, err := os.Stat(p.DesktopProfileDir("claude2")); !os.IsNotExist(err) {
		t.Error("Desktop profile left in place")
	}
	if found, _ := filepath.Glob(filepath.Join(p.BackupsDir(), "remove-claude2-*", "desktop", "config.json")); len(found) != 1 {
		t.Errorf("Desktop profile not in backup: %v", found)
	}
	s, _ = desktop.LoadState(p)
	if _, ok := s.Profiles["claude2"]; ok {
		t.Error("Desktop state still lists claude2")
	}
}

func TestAddCleansUpWhenLoginNotCompleted(t *testing.T) {
	p := addSetup(t)
	fakeClaude(t, "")
	code, _, stderr := run(t, "add", "work")
	if code != 1 || !strings.Contains(stderr, "login did not complete") {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(p.Home, ".claude-work")); !os.IsNotExist(err) {
		t.Error("half-made config dir left in place")
	}
	if found, _ := filepath.Glob(filepath.Join(p.BackupsDir(), "add-failed-work-*")); len(found) != 1 {
		t.Errorf("half-made dir not moved to backup: %v", found)
	}
}

func TestPanelFlagsLoginProblems(t *testing.T) {
	p := twoAccounts(t)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claude3"), "c3", "o3", nil)
	run(t, "init")
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	testutil.WriteFile(t, filepath.Join(p.Home, ".claude2", ".claude.json"), `{"oauthAccount":{"accountUuid":"zz","organizationUuid":"o1","emailAddress":"other@example.com"}}`)
	testutil.WriteFile(t, filepath.Join(p.Home, ".claude3", ".claude.json"), `{"numStartups":3}`)
	_, v := panel(t)
	if v.Accounts[0].LoginProblem != "" {
		t.Errorf("claude: %q", v.Accounts[0].LoginProblem)
	}
	if !strings.Contains(v.Accounts[1].LoginProblem, "different Claude account") {
		t.Errorf("claude2: %q", v.Accounts[1].LoginProblem)
	}
	if v.Accounts[2].LoginProblem != "not logged in" {
		t.Errorf("claude3: %q", v.Accounts[2].LoginProblem)
	}
}
