package usage_test

import (
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/usage"
)

var reg3 = account.Registry{Accounts: []account.Account{{Name: "a"}, {Name: "b"}, {Name: "c"}}}

func w(pct float64, resets int64) *usage.Window {
	return &usage.Window{UsedPercent: pct, ResetsAt: resets}
}

func TestRecommendLowestUsage(t *testing.T) {
	now := time.Unix(1000, 0)
	recs := map[string]usage.Record{
		"b": {FiveHour: w(70, 5000), SevenDay: w(10, 9000)},
		"c": {FiveHour: w(20, 5000), SevenDay: w(40, 9000)},
	}
	if got, ok := usage.Recommend(reg3, recs, "a", now); !ok || got != "c" {
		t.Errorf("got %q %v, want c", got, ok)
	}
}

func TestRecommendSkipsLimited(t *testing.T) {
	now := time.Unix(1000, 0)
	recs := map[string]usage.Record{
		"b": {Limit: &usage.LimitHit{ResetsAt: 2000}},
		"c": {SevenDay: w(95, 9000), Limit: &usage.LimitHit{ResetsAt: 0, ObservedAt: 900}},
	}
	if got, ok := usage.Recommend(reg3, recs, "a", now); ok {
		t.Errorf("got %q, want none (b limited until 2000, c limited recently)", got)
	}
	recs["c"] = usage.Record{SevenDay: w(95, 9000), Limit: &usage.LimitHit{ObservedAt: 1000 - 6*3600}}
	if got, ok := usage.Recommend(reg3, recs, "a", now); !ok || got != "c" {
		t.Errorf("stale unknown-reset limit should not block: got %q %v", got, ok)
	}
}

func TestRecommendResetWindowCountsAsZero(t *testing.T) {
	now := time.Unix(1000, 0)
	recs := map[string]usage.Record{
		"b": {SevenDay: w(99, 500)},
		"c": {SevenDay: w(30, 9000)},
	}
	if got, _ := usage.Recommend(reg3, recs, "a", now); got != "b" {
		t.Errorf("got %q, want b (its 99%% window already reset)", got)
	}
}

func TestRecommendNoneAvailable(t *testing.T) {
	one := account.Registry{Accounts: []account.Account{{Name: "a"}}}
	if got, ok := usage.Recommend(one, nil, "a", time.Now()); ok {
		t.Errorf("got %q", got)
	}
}
