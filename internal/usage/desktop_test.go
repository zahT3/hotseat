package usage_test

import (
	"os"
	"path/filepath"
	"testing"

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
	if s := got["o1"]; s.T != 3000 || s.FiveHour != 0 || s.SevenDay != 100 {
		t.Errorf("o1 = %+v", s)
	}
	if s := got["o2"]; s.T != 2000 || s.FiveHour != 16 {
		t.Errorf("o2 = %+v", s)
	}
	if m, err := usage.ReadDesktopSamples(filepath.Join(t.TempDir(), "none.json")); err != nil || len(m) != 0 {
		t.Errorf("missing file: %v %v", m, err)
	}
}

func TestMergeDesktopWhenNewer(t *testing.T) {
	rec := usage.Record{SevenDay: &usage.Window{UsedPercent: 40, ResetsAt: 9000, ObservedAt: 1000, Source: "statusline"}}
	got := usage.MergeDesktop(rec, &usage.Sample{T: 2000, FiveHour: 5, SevenDay: 60})
	if got.SevenDay.UsedPercent != 60 || got.SevenDay.ResetsAt != 9000 || got.SevenDay.Source != "desktop" {
		t.Errorf("seven day = %+v", got.SevenDay)
	}
	if got.FiveHour == nil || got.FiveHour.UsedPercent != 5 || got.FiveHour.ResetsAt != 0 {
		t.Errorf("five hour = %+v", got.FiveHour)
	}
	older := usage.MergeDesktop(rec, &usage.Sample{T: 500, SevenDay: 1})
	if older.SevenDay.UsedPercent != 40 {
		t.Errorf("older desktop sample won: %+v", older.SevenDay)
	}
}
