package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
)

// desktopSetup registers claude (a1/o1) and claude2 (b2/o2), puts claude's
// Desktop data at the default path and records it as adopted.
func desktopSetup(t *testing.T) paths.Paths {
	t.Helper()
	p := twoAccounts(t)
	run(t, "init")
	writeDesktopDir(t, p.DesktopDataDir(), "a1")
	desktop.SaveState(p, desktop.State{Active: "claude", Profiles: map[string]string{"claude": "a1"}})
	return p
}

func writeDesktopDir(t *testing.T, dir, acct string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, "config.json"), `{"oauth:tokenCache":"SECRET-TOKEN","lastKnownAccountUuid":"`+acct+`"}`)
	testutil.WriteFile(t, filepath.Join(dir, "Cookies"), "SECRET-COOKIE")
}

func TestDesktopStatusOK(t *testing.T) {
	p := desktopSetup(t)
	writeDesktopDir(t, p.DesktopProfileDir("claude2"), "b2")
	s, _ := desktop.LoadState(p)
	s.Profiles["claude2"] = "b2"
	desktop.SaveState(p, s)
	code, out, stderr := run(t, "desktop", "status")
	if code != 0 || !strings.Contains(out, "claude") || !strings.Contains(out, "claude2") || strings.Contains(out, "mismatch") {
		t.Errorf("exit %d out %q err %q", code, out, stderr)
	}
}

func TestDesktopStatusFlagsMismatch(t *testing.T) {
	p := desktopSetup(t)
	writeDesktopDir(t, p.DesktopProfileDir("claude2"), "zz")
	s, _ := desktop.LoadState(p)
	s.Profiles["claude2"] = "b2"
	desktop.SaveState(p, s)
	code, out, _ := run(t, "desktop", "status")
	if code != 1 || !strings.Contains(out, "mismatch") {
		t.Errorf("exit %d out %q", code, out)
	}
}

func TestDesktopStatusNeverPrintsTokens(t *testing.T) {
	desktopSetup(t)
	for _, args := range [][]string{{"desktop", "status"}, {"desktop", "status", "--json"}} {
		_, out, stderr := run(t, args...)
		if strings.Contains(out+stderr, "SECRET") {
			t.Errorf("%v leaked a credential: %q", args, out+stderr)
		}
	}
	_ = os.Remove
}

type fakeDesktop struct {
	running    []desktop.Proc
	quits      int
	urls       []string
	opened     int
	stopOnQuit bool
}

func (f *fakeDesktop) Procs() ([]desktop.Proc, error) { return f.running, nil }
func (f *fakeDesktop) Quit() error {
	f.quits++
	if f.stopOnQuit {
		f.running = nil
	}
	return nil
}
func (f *fakeDesktop) OpenURL(u string) error { f.urls = append(f.urls, u); return nil }
func (f *fakeDesktop) Launch() error          { f.opened++; return nil }

var desktopApp = []desktop.Proc{{PID: 7, Exe: "/Applications/Claude.app/Contents/MacOS/Claude"}}

func TestAdoptActive(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	writeDesktopDir(t, p.DesktopDataDir(), "a1")
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	if code, _, stderr := run(t, "desktop", "adopt"); code != 0 {
		t.Fatalf("adopt: %s", stderr)
	}
	s, _ := desktop.LoadState(p)
	if s.Active != "claude" || s.Profiles["claude"] != "a1" {
		t.Errorf("state %+v", s)
	}
}

func TestAdoptActiveUnknownAccount(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	writeDesktopDir(t, p.DesktopDataDir(), "zz")
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	if code, _, stderr := run(t, "desktop", "adopt"); code != 1 || !strings.Contains(stderr, "zz") {
		t.Errorf("exit %d stderr %q", code, stderr)
	}
}

func TestAdoptDirMovesIntoStore(t *testing.T) {
	p := desktopSetup(t)
	ext := filepath.Join(t.TempDir(), "userdata-b")
	writeDesktopDir(t, ext, "b2")
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	if code, _, stderr := run(t, "desktop", "adopt", "claude2", ext); code != 0 {
		t.Fatalf("adopt: %s", stderr)
	}
	if _, err := os.Stat(ext); err == nil {
		t.Error("source dir still in place")
	}
	if _, err := os.Stat(filepath.Join(p.DesktopProfileDir("claude2"), "config.json")); err != nil {
		t.Error("not moved into the store")
	}
	if s, _ := desktop.LoadState(p); s.Profiles["claude2"] != "b2" {
		t.Errorf("state %+v", s)
	}
}

func TestAdoptDirWrongAccountRefused(t *testing.T) {
	desktopSetup(t)
	ext := filepath.Join(t.TempDir(), "userdata-x")
	writeDesktopDir(t, ext, "zz")
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	if code, _, _ := run(t, "desktop", "adopt", "claude2", ext); code != 1 {
		t.Errorf("exit %d", code)
	}
	if _, err := os.Stat(ext); err != nil {
		t.Error("refused dir was moved anyway")
	}
}

func TestAdoptDirRefusedWhileDesktopRuns(t *testing.T) {
	desktopSetup(t)
	ext := filepath.Join(t.TempDir(), "userdata-b")
	writeDesktopDir(t, ext, "b2")
	defer cli.SetDesktopDriver(&fakeDesktop{running: desktopApp})()
	if code, _, stderr := run(t, "desktop", "adopt", "claude2", ext); code != 1 || !strings.Contains(stderr, "running") {
		t.Errorf("exit %d stderr %q", code, stderr)
	}
}

func TestDesktopAddCreatesEmptyProfileAndLaunches(t *testing.T) {
	p := desktopSetup(t)
	d := &fakeDesktop{}
	defer cli.SetDesktopDriver(d)()
	code, out, stderr := run(t, "desktop", "add", "claude2")
	if code != 0 {
		t.Fatalf("add: %s", stderr)
	}
	s, _ := desktop.LoadState(p)
	if s.Active != "claude2" || d.opened != 1 || !strings.Contains(out, "log in") {
		t.Errorf("state %+v opened %d out %q", s, d.opened, out)
	}
	if id, _ := desktop.AccountUUID(p.DesktopProfileDir("claude")); id != "a1" {
		t.Errorf("claude's data not parked: %q", id)
	}
}

func TestDesktopSwitchCommand(t *testing.T) {
	p := desktopSetup(t)
	writeDesktopDir(t, p.DesktopProfileDir("claude2"), "b2")
	s, _ := desktop.LoadState(p)
	s.Profiles["claude2"] = "b2"
	desktop.SaveState(p, s)
	d := &fakeDesktop{running: desktopApp, stopOnQuit: true}
	defer cli.SetDesktopDriver(d)()
	code, _, stderr := run(t, "desktop", "switch", "claude2", "--resume", "11111111-2222-4333-8444-555555555555")
	if code != 0 {
		t.Fatalf("switch: %s", stderr)
	}
	if len(d.urls) != 1 || !strings.HasSuffix(d.urls[0], "11111111-2222-4333-8444-555555555555") {
		t.Errorf("urls %v", d.urls)
	}
}

func TestDesktopRecoverCommand(t *testing.T) {
	desktopSetup(t)
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	if code, out, _ := run(t, "desktop", "recover"); code != 0 || !strings.Contains(out, "nothing") {
		t.Errorf("exit %d out %q", code, out)
	}
}
