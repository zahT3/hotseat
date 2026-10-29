package desktop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
)

func writeConfig(t *testing.T, dir, acct string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCache":"SECRET-TOKEN","lastKnownAccountUuid":"`+acct+`"}`)
}

func focusIndex(t *testing.T, dir, acct, org, id, cli string, focused int64) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"sessionId": "local_" + id, "cliSessionId": cli, "lastFocusedAt": focused, "lastActivityAt": focused})
	testutil.WriteFile(t, filepath.Join(dir, "claude-code-sessions", acct, org, "local_"+id+".json"), string(b))
}

func switchSetup(t *testing.T) (paths.Paths, account.Registry) {
	t.Helper()
	p := testutil.FakeHome(t)
	reg := account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true, AccountUUID: "a1", OrgUUID: "o1"},
		{Name: "claude2", ConfigDir: filepath.Join(p.Home, ".claude2"), AccountUUID: "b2", OrgUUID: "o2"},
	}}
	active := p.DesktopDataDir()
	writeConfig(t, active, "a1")
	focusIndex(t, active, "a1", "o1", "S", "cs", 10)
	focusIndex(t, active, "a1", "o1", "T", "ct", 5)
	testutil.WriteFile(t, filepath.Join(active, "claude_desktop_config.json"), `{"mcpServers":{"m":{"command":"x"}},"preferences":{}}`)
	writeConfig(t, p.DesktopProfileDir("claude2"), "b2")
	for _, id := range []string{"cs", "ct"} {
		testutil.WriteFile(t, filepath.Join(p.CoreDir(), "projects", "-x", id+".jsonl"), "{}")
	}
	desktop.SaveState(p, desktop.State{Active: "claude", Profiles: map[string]string{"claude": "a1", "claude2": "b2"}})
	return p, reg
}

func acctOf(t *testing.T, dir string) string {
	t.Helper()
	id, _ := desktop.AccountUUID(dir)
	return id
}

func TestCurrentSession(t *testing.T) {
	p, _ := switchSetup(t)
	if id, ok := desktop.CurrentSession(p.DesktopDataDir(), "a1", "o1"); !ok || id != "cs" {
		t.Errorf("got %q %v", id, ok)
	}
}

func TestSwitchSyncsSwapsAndResumes(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{procs: claudeApp, stopsOn: 1}
	res, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Resume: "current", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if acctOf(t, p.DesktopDataDir()) != "b2" || acctOf(t, p.DesktopProfileDir("claude")) != "a1" {
		t.Errorf("real path=%s parked claude=%s", acctOf(t, p.DesktopDataDir()), acctOf(t, p.DesktopProfileDir("claude")))
	}
	for _, id := range []string{"S", "T"} {
		if _, err := os.Stat(filepath.Join(p.DesktopDataDir(), "claude-code-sessions", "b2", "o2", "local_"+id+".json")); err != nil {
			t.Errorf("session %s not in claude2's list", id)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(p.DesktopDataDir(), "claude_desktop_config.json")); !strings.Contains(string(b), `"m"`) {
		t.Errorf("MCP not synced: %s", b)
	}
	if len(d.urls) != 1 || d.urls[0] != "claude://resume?session=cs" || d.quits != 1 {
		t.Errorf("urls %v quits %d", d.urls, d.quits)
	}
	if s, _ := desktop.LoadState(p); s.Active != "claude2" || res.To != "claude2" {
		t.Errorf("state %+v result %+v", s, res)
	}
}

func TestSwitchRefusesWhileRunning(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{procs: claudeApp}
	_, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Timeout: 600 * time.Millisecond})
	if err == nil || acctOf(t, p.DesktopDataDir()) != "a1" || acctOf(t, p.DesktopProfileDir("claude2")) != "b2" {
		t.Errorf("err %v; real path=%s", err, acctOf(t, p.DesktopDataDir()))
	}
}

func TestSwitchRefusesIdentityMismatch(t *testing.T) {
	p, reg := switchSetup(t)
	writeConfig(t, p.DesktopDataDir(), "zz")
	_, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "zz") || acctOf(t, p.DesktopProfileDir("claude2")) != "b2" {
		t.Errorf("err %v", err)
	}
}

func TestSwitchNoopSameAccount(t *testing.T) {
	p, reg := switchSetup(t)
	d := &fakeDriver{}
	if _, err := desktop.Switch(p, reg, d, "claude", desktop.SwitchOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	if d.quits != 0 || len(d.urls) != 0 || d.opened != 0 {
		t.Errorf("driver used: %+v", d)
	}
}

func TestSwitchToMissingProfile(t *testing.T) {
	p, reg := switchSetup(t)
	reg.Accounts = append(reg.Accounts, account.Account{Name: "claude3", AccountUUID: "c3", OrgUUID: "o3"})
	_, err := desktop.Switch(p, reg, &fakeDriver{}, "claude3", desktop.SwitchOptions{Timeout: time.Second})
	if err == nil || !strings.Contains(err.Error(), "desktop add claude3") {
		t.Errorf("err %v", err)
	}
}

func TestSwitchFreshProfileLearnsIdentity(t *testing.T) {
	p, reg := switchSetup(t)
	desktop.SaveState(p, desktop.State{Active: "claude", Profiles: map[string]string{"claude": "", "claude2": "b2"}})
	if _, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	if s, _ := desktop.LoadState(p); s.Profiles["claude"] != "a1" {
		t.Errorf("identity of the fresh profile not recorded: %+v", s)
	}
}
