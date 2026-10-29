package desktop_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/testutil"
)

func crashAt(t *testing.T, step string) {
	desktop.SetBeforeSwapStep(t, func(s string) error {
		if s == step {
			return errors.New("simulated crash at " + step)
		}
		return nil
	})
}

func TestRecoverUpdatesStateWhenFinishing(t *testing.T) {
	p, reg := switchSetup(t)
	crashAt(t, "activate")
	if _, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second}); err == nil {
		t.Fatal("expected the simulated crash")
	}
	desktop.SetBeforeSwapStep(t, nil)
	if _, err := desktop.Recover(p); err != nil {
		t.Fatal(err)
	}
	if s, _ := desktop.LoadState(p); s.Active != "claude2" || acctOf(t, p.DesktopDataDir()) != "b2" {
		t.Errorf("state %+v, real path %s", s, acctOf(t, p.DesktopDataDir()))
	}
	if _, err := desktop.Switch(p, reg, &fakeDriver{}, "claude", desktop.SwitchOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	if acctOf(t, p.DesktopDataDir()) != "a1" {
		t.Errorf("switch back after recover did not move data: real path %s", acctOf(t, p.DesktopDataDir()))
	}
}

func TestSwapCleanupFailureStillSavesState(t *testing.T) {
	p, reg := switchSetup(t)
	crashAt(t, "cleanup")
	desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second})
	if s, _ := desktop.LoadState(p); s.Active != "claude2" {
		t.Errorf("state %+v after a swap whose only failure was journal cleanup", s)
	}
}

func TestNoopRefusesWhenRealPathDrifted(t *testing.T) {
	p, reg := switchSetup(t)
	writeConfig(t, p.DesktopDataDir(), "b2")
	res, err := desktop.Switch(p, reg, &fakeDriver{}, "claude", desktop.SwitchOptions{Timeout: time.Second})
	if err == nil || res.Noop || !strings.Contains(err.Error(), "b2") {
		t.Errorf("res %+v err %v", res, err)
	}
}

func TestSwitchRefusesTargetLoggedInElsewhere(t *testing.T) {
	p, reg := switchSetup(t)
	writeConfig(t, p.DesktopProfileDir("claude2"), "zz")
	d := &fakeDriver{}
	if _, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Timeout: time.Second}); err == nil || !strings.Contains(err.Error(), "zz") {
		t.Fatalf("err %v", err)
	}
	if acctOf(t, p.DesktopDataDir()) != "a1" || acctOf(t, p.DesktopProfileDir("claude2")) != "zz" {
		t.Error("dirs moved despite the mismatch")
	}
}

func TestAddProfileRefusesLeftoverDir(t *testing.T) {
	p, _ := switchSetup(t)
	testutil.WriteFile(t, filepath.Join(p.DesktopProfileDir("claude3"), "leftover"), "x")
	if err := desktop.AddProfile(p, "claude3"); err == nil {
		t.Error("a leftover dir was treated as a fresh profile")
	}
}

func TestSwitchRelaunchesAfterFailure(t *testing.T) {
	p, reg := switchSetup(t)
	testutil.WriteFile(t, filepath.Join(p.DesktopDataDir(), "claude_desktop_config.json"), "{")
	d := &fakeDriver{procs: claudeApp, stopsOn: 1}
	_, err := desktop.Switch(p, reg, d, "claude2", desktop.SwitchOptions{Timeout: 2 * time.Second})
	if err == nil || d.opened != 1 || acctOf(t, p.DesktopDataDir()) != "a1" {
		t.Errorf("err %v opened %d real %s", err, d.opened, acctOf(t, p.DesktopDataDir()))
	}
}

func TestSwitchRefusesEmptyOrgUUID(t *testing.T) {
	p, reg := switchSetup(t)
	reg.Accounts[1].OrgUUID = ""
	if _, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second}); err == nil {
		t.Error("expected refusal for an account without an organization ID")
	}
}

func TestSwitchWaitsForLock(t *testing.T) {
	p, reg := switchSetup(t)
	unlock, err := desktop.Lock(p)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := desktop.Switch(p, reg, &fakeDriver{}, "claude2", desktop.SwitchOptions{Timeout: time.Second})
		done <- err
	}()
	time.Sleep(300 * time.Millisecond)
	if acctOf(t, p.DesktopDataDir()) != "a1" {
		t.Fatal("switch ran while another holder had the lock")
	}
	unlock()
	if err := <-done; err != nil || acctOf(t, p.DesktopDataDir()) != "b2" {
		t.Errorf("err %v real %s", err, acctOf(t, p.DesktopDataDir()))
	}
}

func TestRemapNullNoPanic(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, cfg), `{"preferences":{"xByAccount":{"a1":true}}}`)
	testutil.WriteFile(t, filepath.Join(dst, cfg), `{"preferences":{"xByAccount":null}}`)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	if err := desktop.SyncSettings(src, dst, "a1", "b2", bk); err != nil {
		t.Fatal(err)
	}
}

func TestScratchNewerSourceWins(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	rel := filepath.Join("scratch-workspaces", "a1", "o1", "s1", "main.py")
	testutil.WriteFile(t, filepath.Join(dst, rel), "old")
	testutil.WriteFile(t, filepath.Join(src, rel), "new")
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dst, rel), old, old)
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	if b, _ := os.ReadFile(filepath.Join(dst, rel)); string(b) != "new" {
		t.Errorf("edited scratch file did not carry over: %q", b)
	}
	if found, _ := filepath.Glob(filepath.Join(bk, "*", "main.py")); len(found) != 1 {
		t.Errorf("replaced scratch file not backed up: %v", found)
	}
}

func TestIdenticalExtensionsNotBackedUp(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	for _, d := range []string{src, dst} {
		testutil.WriteFile(t, filepath.Join(d, "Claude Extensions", "e1", "manifest.json"), "{}")
	}
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	if entries, _ := os.ReadDir(bk); len(entries) != 0 {
		t.Errorf("identical extensions backed up: %v", entries)
	}
}

func TestRunningUsesRealPaths(t *testing.T) {
	procs := []desktop.Proc{
		{PID: 1, Exe: "claude", Paths: []string{home + "/Library/Application Support/Claude/claude-code/2.1.309/claude"}},
		{PID: 2, Exe: "claude bg-pty-host", Paths: []string{home + "/Library/Application Support/Claude/scratch-workspaces/a/b"}},
		{PID: 3, Exe: "claude", Paths: []string{home + "/.local/share/claude/versions/2.1.310", home + "/proj"}},
	}
	got := desktop.Running(procs, home)
	if len(got) != 2 || got[0].PID != 1 || got[1].PID != 2 {
		t.Errorf("got %v", got)
	}
}

var _ = account.Account{}
