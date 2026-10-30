package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/testutil"
	"github.com/zahT3/hotseat/internal/usage"
)

type panelOut struct {
	Current   string `json:"current"`
	Recommend string `json:"recommend"`
	Desktop   struct {
		Managed         bool   `json:"managed"`
		Active          string `json:"active"`
		Running         bool   `json:"running"`
		RunningKnown    bool   `json:"runningKnown"`
		PendingRecovery bool   `json:"pendingRecovery"`
	} `json:"desktop"`
	Accounts []struct {
		Account  string `json:"account"`
		Current  bool   `json:"current"`
		FiveHour *struct {
			UsedPercent float64 `json:"usedPercent"`
		} `json:"fiveHour"`
		Desktop        string `json:"desktop"`
		DesktopProblem string `json:"desktopProblem"`
		Email          string `json:"email"`
		ConfigDir      string `json:"configDir"`
		Note           string `json:"note"`
	} `json:"accounts"`
}

func panel(t *testing.T) (int, panelOut) {
	t.Helper()
	code, out, stderr := run(t, "panel", "--json")
	var v panelOut
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("exit %d, bad json %q (stderr %q): %v", code, out, stderr, err)
	}
	return code, v
}

func TestPanelJSONShape(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	now := time.Unix(1_900_000_000, 0)
	defer cli.SetClock(func() time.Time { return now })()
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	usage.Save(p, usage.Record{Account: "claude2",
		FiveHour: &usage.Window{UsedPercent: 16, ResetsAt: now.Unix() + 3600, ObservedAt: now.Unix() - 60, Source: "statusline"}})
	code, v := panel(t)
	if code != 0 || v.Current != "claude" || len(v.Accounts) != 2 {
		t.Fatalf("exit %d, %+v", code, v)
	}
	if v.Accounts[0].Account != "claude" || !v.Accounts[0].Current || v.Accounts[1].Account != "claude2" {
		t.Errorf("accounts %+v", v.Accounts)
	}
	if v.Accounts[1].ConfigDir != filepath.Join(p.Home, ".claude2") {
		t.Errorf("claude2 configDir %q", v.Accounts[1].ConfigDir)
	}
	if v.Accounts[1].FiveHour == nil || v.Accounts[1].FiveHour.UsedPercent != 16 {
		t.Errorf("claude2 fiveHour %+v", v.Accounts[1].FiveHour)
	}
	if v.Desktop.Managed {
		t.Errorf("desktop %+v, want unmanaged", v.Desktop)
	}
	for _, a := range v.Accounts {
		if a.Desktop != "none" {
			t.Errorf("%s desktop = %q, want none", a.Account, a.Desktop)
		}
	}
}

func TestPanelDesktopStates(t *testing.T) {
	desktopSetup(t)
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	if code, _, stderr := run(t, "desktop", "add", "claude2"); code != 0 {
		t.Fatalf("desktop add: %s", stderr)
	}
	defer cli.SetDesktopDriver(&fakeDesktop{running: desktopApp})()
	code, v := panel(t)
	// desktop add switches Desktop to the new, empty profile.
	if code != 0 || !v.Desktop.Managed || v.Desktop.Active != "claude2" || !v.Desktop.Running || !v.Desktop.RunningKnown {
		t.Fatalf("exit %d desktop %+v", code, v.Desktop)
	}
	if v.Accounts[0].Desktop != "parked" || v.Accounts[0].DesktopProblem != "" {
		t.Errorf("claude %+v", v.Accounts[0])
	}
	if v.Accounts[1].Desktop != "active" || v.Accounts[1].DesktopProblem != "not logged in yet" {
		t.Errorf("claude2 %+v", v.Accounts[1])
	}
}

func TestPanelPendingRecovery(t *testing.T) {
	p := desktopSetup(t)
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	testutil.WriteFile(t, p.DesktopJournal(), `{"from":"claude","to":"claude2","step":"start"}`)
	code, v := panel(t)
	if code != 0 || !v.Desktop.PendingRecovery {
		t.Errorf("exit %d desktop %+v", code, v.Desktop)
	}
}

func TestPanelRecommendSkipsCurrentAndLimited(t *testing.T) {
	p := twoAccounts(t)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claude3"), "c3", "o3", nil)
	run(t, "init")
	now := time.Unix(1_900_000_000, 0)
	defer cli.SetClock(func() time.Time { return now })()
	defer cli.SetDesktopDriver(&fakeDesktop{})()
	usage.SetLimit(p, "claude2", usage.LimitHit{Message: "limit", ResetsAt: now.Unix() + 3600, ObservedAt: now.Unix()})
	code, v := panel(t)
	if code != 0 || v.Recommend != "claude3" {
		t.Errorf("exit %d recommend %q", code, v.Recommend)
	}
}
