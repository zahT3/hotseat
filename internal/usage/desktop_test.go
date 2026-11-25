package usage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/usage"
)

func TestReadDesktopSamplesLatestPerOrg(t *testing.T) {
	f := filepath.Join(t.TempDir(), "plan-usage-history.json")
	os.WriteFile(f, []byte(`{"version":1,"samples":[
	  {"t":1000000,"org":"o1","u":{"fh":10,"sd":20}},
	  {"t":3000000,"org":"o1","u":{"fh":0,"sd":100}},
	  {"t":2000000,"org":"o2","u":{"fh":16,"sd":12}}]}`), 0o600)
	got, err := usage.ReadDesktopSamples(f)
	if err != nil {
		t.Fatal(err)
	}
	if s := got["o1"]; s.T != 3000 || s.FiveHour == nil || *s.FiveHour != 0 || *s.SevenDay != 100 {
		t.Errorf("o1 = %+v", s)
	}
	if s := got["o2"]; s.T != 2000 || *s.FiveHour != 16 {
		t.Errorf("o2 = %+v", s)
	}
	if m, err := usage.ReadDesktopSamples(filepath.Join(t.TempDir(), "none.json")); err != nil || len(m) != 0 {
		t.Errorf("missing file: %v %v", m, err)
	}
}

func TestMergeDesktopWhenNewer(t *testing.T) {
	rec := usage.Record{SevenDay: &usage.Window{UsedPercent: 40, ResetsAt: 9000, ObservedAt: 1000, Source: "statusline"}}
	got := usage.MergeDesktop(rec, &usage.Sample{T: 2000, FiveHour: pct(5), SevenDay: pct(60)})
	if got.SevenDay.UsedPercent != 60 || got.SevenDay.ResetsAt != 9000 || got.SevenDay.Source != "desktop" {
		t.Errorf("seven day = %+v", got.SevenDay)
	}
	if got.FiveHour == nil || got.FiveHour.UsedPercent != 5 || got.FiveHour.ResetsAt != 0 {
		t.Errorf("five hour = %+v", got.FiveHour)
	}
	older := usage.MergeDesktop(rec, &usage.Sample{T: 500, SevenDay: pct(1)})
	if older.SevenDay.UsedPercent != 40 {
		t.Errorf("older desktop sample won: %+v", older.SevenDay)
	}
}

// Desktop often records only the weekly number ({"sd": 52}). A missing
// 5-hour value must stay unknown instead of showing as 0%.
func TestMergeDesktopMissingFiveHourStaysUnknown(t *testing.T) {
	f := filepath.Join(t.TempDir(), "plan-usage-history.json")
	os.WriteFile(f, []byte(`{"samples":[{"t":2000000,"org":"o1","u":{"sd":52}}]}`), 0o600)
	got, err := usage.ReadDesktopSamples(f)
	if err != nil {
		t.Fatal(err)
	}
	s := got["o1"]
	r := usage.MergeDesktop(usage.Record{}, &s)
	if r.FiveHour != nil {
		t.Errorf("five hour = %+v, want unknown", r.FiveHour)
	}
	if r.SevenDay == nil || r.SevenDay.UsedPercent != 52 {
		t.Errorf("seven day = %+v", r.SevenDay)
	}
	// An older statusline reading stays when Desktop has no newer one.
	five := &usage.Window{UsedPercent: 30, ObservedAt: 1000, Source: "statusline"}
	if r := usage.MergeDesktop(usage.Record{FiveHour: five}, &s); r.FiveHour != five {
		t.Errorf("five hour replaced: %+v", r.FiveHour)
	}
}

func pct(v float64) *float64 { return &v }

// Desktop gives no reset times, so age is all there is to go by: a reading
// older than its window says nothing about the window running now.
func TestSampleFreshDropsReadingsOlderThanTheirWindow(t *testing.T) {
	at := time.Unix(1_900_000_000, 0)
	s := usage.Sample{T: at.Unix(), FiveHour: pct(95), SevenDay: pct(40)}
	if f := s.Fresh(at.Add(4 * time.Hour)); f.FiveHour == nil || f.SevenDay == nil {
		t.Errorf("4h old: %+v", f)
	}
	if f := s.Fresh(at.Add(6 * time.Hour)); f.FiveHour != nil || f.SevenDay == nil || *f.SevenDay != 40 {
		t.Errorf("6h old: five hour should be unknown, seven day kept: %+v", f)
	}
	if f := s.Fresh(at.Add(8 * 24 * time.Hour)); f.FiveHour != nil || f.SevenDay != nil {
		t.Errorf("8d old: %+v", f)
	}
}
