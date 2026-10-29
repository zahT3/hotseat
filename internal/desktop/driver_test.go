package desktop_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/desktop"
)

const home = "/Users/u"

func TestRunningIgnoresChromeNativeHost(t *testing.T) {
	procs := []desktop.Proc{{PID: 1, Exe: "/Applications/Claude.app/Contents/Helpers/chrome-native-host"}}
	if got := desktop.Running(procs, home); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestRunningCountsHelpersAndBundledCLI(t *testing.T) {
	procs := []desktop.Proc{
		{PID: 1, Exe: "/Applications/Claude.app/Contents/MacOS/Claude"},
		{PID: 2, Exe: "/Applications/Claude.app/Contents/Frameworks/Claude Helper (Renderer).app/Contents/MacOS/Claude Helper (Renderer)"},
		{PID: 3, Exe: home + "/Library/Application Support/Claude/claude-code/2.1.309/claude"},
		{PID: 4, Exe: "/private/var/folders/x/T/com.anthropic.claudefordesktop.ShipIt.y/Claude.app/Contents/Helpers/disclaimer"},
	}
	if got := desktop.Running(procs, home); len(got) != 4 {
		t.Errorf("got %v", got)
	}
}

func TestRunningIgnoresTerminalClaude(t *testing.T) {
	procs := []desktop.Proc{
		{PID: 1, Exe: home + "/.local/share/claude/versions/2.1.310"},
		{PID: 2, Exe: home + "/.local/share/claude/ClaudeCode.app/Contents/MacOS/claude"},
		{PID: 3, Exe: "claude"},
		{PID: 4, Exe: home + "/Library/Application Support/Claude-3p/x"},
	}
	if got := desktop.Running(procs, home); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

type fakeDriver struct {
	procs   []desktop.Proc
	quits   int
	stopsOn int // number of Procs calls after which the app is gone (0 = never)
	calls   int
	urls    []string
	opened  int
}

func (f *fakeDriver) Procs() ([]desktop.Proc, error) {
	f.calls++
	if f.stopsOn > 0 && f.calls > f.stopsOn {
		return nil, nil
	}
	return f.procs, nil
}
func (f *fakeDriver) Quit() error            { f.quits++; return nil }
func (f *fakeDriver) OpenURL(u string) error { f.urls = append(f.urls, u); return nil }
func (f *fakeDriver) Launch() error          { f.opened++; return nil }

var claudeApp = []desktop.Proc{{PID: 42, Exe: "/Applications/Claude.app/Contents/MacOS/Claude"}}

func TestQuitAndWaitTimesOut(t *testing.T) {
	d := &fakeDriver{procs: claudeApp}
	err := desktop.QuitAndWait(d, home, 600*time.Millisecond)
	if !errors.Is(err, desktop.ErrStillRunning) || !strings.Contains(err.Error(), "42") || d.quits != 1 {
		t.Errorf("err %v quits %d", err, d.quits)
	}
}

func TestQuitAndWaitSucceeds(t *testing.T) {
	d := &fakeDriver{procs: claudeApp, stopsOn: 2}
	if err := desktop.QuitAndWait(d, home, 3*time.Second); err != nil || d.quits != 1 {
		t.Errorf("err %v quits %d", err, d.quits)
	}
	idle := &fakeDriver{}
	if err := desktop.QuitAndWait(idle, home, time.Second); err != nil || idle.quits != 0 {
		t.Errorf("already quit: err %v quits %d", err, idle.quits)
	}
}
