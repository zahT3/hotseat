package usage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/testutil"
	"github.com/zahT3/hotseat/internal/usage"
)

func win(pct float64, resets int64) *usage.RateWindow {
	return &usage.RateWindow{UsedPercentage: pct, ResetsAt: resets}
}

func TestObserveWritesOnlyOnChange(t *testing.T) {
	p := testutil.FakeHome(t)
	now := time.Unix(1_900_000_000, 0)
	rl := usage.RateLimits{FiveHour: win(16, 1_900_010_000), SevenDay: win(12, 1_900_300_000)}
	if err := usage.ObserveRateLimits(p, "claude2", rl, now); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(p.UsageDir(), "claude2.json")
	old := time.Unix(1_000_000_000, 0)
	os.Chtimes(file, old, old)
	if err := usage.ObserveRateLimits(p, "claude2", rl, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(file)
	if !info.ModTime().Equal(old) {
		t.Error("unchanged data rewrote the file")
	}
	r, _ := usage.Load(p, "claude2")
	if r.FiveHour == nil || r.FiveHour.UsedPercent != 16 || r.FiveHour.ResetsAt != 1_900_010_000 || r.FiveHour.Source != "statusline" {
		t.Errorf("five hour = %+v", r.FiveHour)
	}
}

func TestObserveKeepsMissingWindow(t *testing.T) {
	p := testutil.FakeHome(t)
	now := time.Unix(1_900_000_000, 0)
	usage.ObserveRateLimits(p, "a", usage.RateLimits{FiveHour: win(10, 1), SevenDay: win(20, 1_900_300_000)}, now)
	usage.ObserveRateLimits(p, "a", usage.RateLimits{FiveHour: win(30, 1_900_010_000)}, now.Add(time.Minute))
	r, _ := usage.Load(p, "a")
	if r.SevenDay == nil || r.SevenDay.UsedPercent != 20 {
		t.Errorf("seven day lost: %+v", r.SevenDay)
	}
	if r.FiveHour.UsedPercent != 30 {
		t.Errorf("five hour = %+v", r.FiveHour)
	}
}

func TestEffectiveAfterReset(t *testing.T) {
	now := time.Unix(2000, 0)
	pct, known, reset := usage.Effective(&usage.Window{UsedPercent: 90, ResetsAt: 1999}, now)
	if pct != 0 || !known || !reset {
		t.Errorf("got %v %v %v", pct, known, reset)
	}
	pct, known, reset = usage.Effective(&usage.Window{UsedPercent: 90, ResetsAt: 2001}, now)
	if pct != 90 || !known || reset {
		t.Errorf("got %v %v %v", pct, known, reset)
	}
	if _, known, _ := usage.Effective(nil, now); known {
		t.Error("nil window must be unknown")
	}
}

func TestLoadMissingReturnsEmpty(t *testing.T) {
	p := testutil.FakeHome(t)
	r, err := usage.Load(p, "nobody")
	if err != nil || r.Account != "nobody" || r.FiveHour != nil || r.Limit != nil {
		t.Errorf("got %+v, %v", r, err)
	}
}

func TestObserveClearsExpiredLimit(t *testing.T) {
	p := testutil.FakeHome(t)
	now := time.Unix(1_900_000_000, 0)
	usage.Save(p, usage.Record{Account: "a", Limit: &usage.LimitHit{Message: "x", ResetsAt: 1_899_999_000}})
	usage.ObserveRateLimits(p, "a", usage.RateLimits{FiveHour: win(5, 1_900_010_000)}, now)
	r, _ := usage.Load(p, "a")
	if r.Limit != nil {
		t.Errorf("expired limit kept: %+v", r.Limit)
	}
	usage.Save(p, usage.Record{Account: "b", Limit: &usage.LimitHit{Message: "x", ResetsAt: 1_900_500_000}})
	usage.ObserveRateLimits(p, "b", usage.RateLimits{FiveHour: win(5, 1_900_010_000)}, now)
	r, _ = usage.Load(p, "b")
	if r.Limit == nil {
		t.Error("active limit dropped")
	}
}
