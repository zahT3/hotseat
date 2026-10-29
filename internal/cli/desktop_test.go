package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
