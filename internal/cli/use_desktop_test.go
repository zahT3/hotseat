package cli_test

import (
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

func useSetup(t *testing.T) paths.Paths {
	t.Helper()
	p := desktopSetup(t)
	writeDesktopDir(t, p.DesktopProfileDir("claude2"), "b2")
	s, _ := desktop.LoadState(p)
	s.Profiles["claude2"] = "b2"
	desktop.SaveState(p, s)
	return p
}

func activeDesktop(t *testing.T, p paths.Paths) string {
	s, _ := desktop.LoadState(p)
	return s.Active
}

func TestUseSwitchesDesktopWithYes(t *testing.T) {
	p := useSetup(t)
	d := &fakeDesktop{running: desktopApp, stopOnQuit: true}
	defer cli.SetDesktopDriver(d)()
	code, _, stderr := run(t, "use", "claude2", "--yes")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if cur, _ := account.CurrentName(p); cur != "claude2" || activeDesktop(t, p) != "claude2" || d.quits != 1 || d.opened+len(d.urls) != 1 {
		t.Errorf("cli %q desktop %q quits %d opened %d urls %v", cur, activeDesktop(t, p), d.quits, d.opened, d.urls)
	}
}

func TestUseAsksBeforeQuittingDesktop(t *testing.T) {
	p := useSetup(t)
	d := &fakeDesktop{running: desktopApp, stopOnQuit: true}
	defer cli.SetDesktopDriver(d)()
	code, out, _ := run(t, "use", "claude2")
	if code != 3 || !strings.Contains(out, "--yes") || d.quits != 0 || activeDesktop(t, p) != "claude" {
		t.Errorf("exit %d out %q quits %d desktop %q", code, out, d.quits, activeDesktop(t, p))
	}
	if cur, _ := account.CurrentName(p); cur != "claude2" {
		t.Errorf("CLI should still switch: current %q", cur)
	}
}

func TestUseCliOnly(t *testing.T) {
	p := useSetup(t)
	d := &fakeDesktop{}
	defer cli.SetDesktopDriver(d)()
	run(t, "use", "claude2", "--cli-only")
	if activeDesktop(t, p) != "claude" || d.opened != 0 {
		t.Errorf("desktop %q opened %d", activeDesktop(t, p), d.opened)
	}
}

func TestUseNoDesktopProfileHint(t *testing.T) {
	desktopSetup(t)
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	code, out, _ := run(t, "use", "claude2")
	if code != 0 || !strings.Contains(out, "hotseat desktop add claude2") {
		t.Errorf("exit %d out %q", code, out)
	}
}

func TestUseDesktopNotRunningSwitchesWithoutLaunch(t *testing.T) {
	p := useSetup(t)
	d := &fakeDesktop{}
	defer cli.SetDesktopDriver(d)()
	if code, _, stderr := run(t, "use", "claude2"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if activeDesktop(t, p) != "claude2" || d.opened != 0 || len(d.urls) != 0 {
		t.Errorf("desktop %q opened %d urls %v", activeDesktop(t, p), d.opened, d.urls)
	}
}

func TestUseReportsDesktopIdentityDrift(t *testing.T) {
	p := useSetup(t)
	writeDesktopDir(t, p.DesktopDataDir(), "b2")
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	code, out, stderr := run(t, "use", "claude")
	if code != 1 || !strings.Contains(out+stderr, "b2") {
		t.Errorf("exit %d out %q err %q", code, out, stderr)
	}
}
