package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/testutil"
	"github.com/zahT3/hotseat/internal/usage"
)

func usageSetup(t *testing.T) func() {
	t.Helper()
	p := twoAccounts(t)
	run(t, "init")
	now := time.Unix(1_900_000_000, 0)
	usage.Save(p, usage.Record{Account: "claude2",
		FiveHour: &usage.Window{UsedPercent: 16, ResetsAt: now.Unix() + 3600, ObservedAt: now.Unix() - 180, Source: "statusline"},
		SevenDay: &usage.Window{UsedPercent: 90, ResetsAt: now.Unix() - 10, ObservedAt: now.Unix() - 180, Source: "statusline"},
	})
	// Desktop's history for the default account's org (o1), newer than anything else.
	testutil.WriteFile(t, filepath.Join(p.DesktopDataDir(), "plan-usage-history.json"),
		`{"version":1,"samples":[{"t":`+itoa((now.Unix()-60)*1000)+`,"org":"o1","u":{"fh":0,"sd":100}}]}`)
	return cli.SetClock(func() time.Time { return now })
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestUsageShowsResetAsZero(t *testing.T) {
	defer usageSetup(t)()
	_, out, _ := run(t, "usage")
	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "claude2") {
			line = l
		}
	}
	if !strings.Contains(line, "5h 16%") || !strings.Contains(line, "7d 0% (reset)") || !strings.Contains(line, "3m ago") {
		t.Errorf("claude2 line = %q\n%s", line, out)
	}
}

func TestUsageMergesDesktopWhenNewer(t *testing.T) {
	defer usageSetup(t)()
	_, out, _ := run(t, "usage")
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(l, "*")), "claude ") {
			if !strings.Contains(l, "7d 100% (desktop)") {
				t.Errorf("claude line = %q", l)
			}
			return
		}
	}
	t.Errorf("no claude line:\n%s", out)
}

func TestUsageJSON(t *testing.T) {
	defer usageSetup(t)()
	_, out, _ := run(t, "usage", "--json")
	var rows []struct {
		Account  string
		Current  bool
		SevenDay *struct {
			UsedPercent float64
			Reset       bool
			Source      string
		}
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("bad json %q: %v", out, err)
	}
	if rows[0].Account != "claude" || !rows[0].Current || rows[0].SevenDay.Source != "desktop" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].SevenDay == nil || !rows[1].SevenDay.Reset || rows[1].SevenDay.UsedPercent != 0 {
		t.Errorf("row 1 = %+v", rows[1].SevenDay)
	}
}
