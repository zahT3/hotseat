package desktop_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
)

func swapSetup(t *testing.T) paths.Paths {
	t.Helper()
	p := testutil.FakeHome(t)
	testutil.WriteFile(t, filepath.Join(p.DesktopDataDir(), "who"), "A")
	testutil.WriteFile(t, filepath.Join(p.DesktopProfileDir("b"), "who"), "B")
	return p
}

func who(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "who"))
	if err != nil {
		return "<missing>"
	}
	return string(b)
}

func TestSwapMovesDirs(t *testing.T) {
	p := swapSetup(t)
	if err := desktop.Swap(p, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if who(t, p.DesktopDataDir()) != "B" || who(t, p.DesktopProfileDir("a")) != "A" || filepath.Join(p.DesktopProfileDir("b"), "who") == "" {
		t.Errorf("active=%s parked a=%s", who(t, p.DesktopDataDir()), who(t, p.DesktopProfileDir("a")))
	}
	if _, err := os.Stat(p.DesktopProfileDir("b")); err == nil {
		t.Error("b still parked")
	}
	if _, err := os.Stat(p.DesktopJournal()); err == nil {
		t.Error("journal left behind")
	}
}

func TestSwapRefusesWhenTargetMissing(t *testing.T) {
	p := swapSetup(t)
	if err := desktop.Swap(p, "a", "nobody"); err == nil {
		t.Fatal("expected error")
	}
	if who(t, p.DesktopDataDir()) != "A" {
		t.Error("active dir changed")
	}
}

func TestRecoverAfterCrashAfterPark(t *testing.T) {
	p := swapSetup(t)
	desktop.SetBeforeSwapStep(t, func(step string) error {
		if step == "activate" {
			return errors.New("simulated crash")
		}
		return nil
	})
	if err := desktop.Swap(p, "a", "b"); err == nil {
		t.Fatal("expected the simulated crash")
	}
	if _, err := os.Stat(p.DesktopDataDir()); err == nil {
		t.Fatal("test setup: active dir should be parked at this point")
	}
	desktop.SetBeforeSwapStep(t, nil)
	msg, err := desktop.Recover(p)
	if err != nil || who(t, p.DesktopDataDir()) != "B" || !strings.Contains(msg, "b") {
		t.Errorf("recover: %q %v, active=%s", msg, err, who(t, p.DesktopDataDir()))
	}
}

func TestRecoverAfterCrashAtStart(t *testing.T) {
	p := swapSetup(t)
	desktop.SetBeforeSwapStep(t, func(step string) error {
		if step == "park" {
			return errors.New("simulated crash")
		}
		return nil
	})
	desktop.Swap(p, "a", "b")
	desktop.SetBeforeSwapStep(t, nil)
	if _, err := desktop.Recover(p); err != nil {
		t.Fatal(err)
	}
	if who(t, p.DesktopDataDir()) != "A" || who(t, p.DesktopProfileDir("b")) != "B" {
		t.Error("state changed by a swap that never started")
	}
	if _, err := os.Stat(p.DesktopJournal()); err == nil {
		t.Error("journal left behind")
	}
}

func TestRecoverNothingToDo(t *testing.T) {
	p := swapSetup(t)
	if msg, err := desktop.Recover(p); err != nil || msg != "" {
		t.Errorf("got %q %v", msg, err)
	}
}

func TestSwapRefusesWithPendingJournal(t *testing.T) {
	p := swapSetup(t)
	testutil.WriteFile(t, p.DesktopJournal(), `{"from":"a","to":"b","step":"parked"}`)
	if err := desktop.Swap(p, "a", "b"); err == nil || !strings.Contains(err.Error(), "recover") {
		t.Errorf("err = %v", err)
	}
}
