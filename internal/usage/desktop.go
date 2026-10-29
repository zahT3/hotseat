package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Sample is Claude Desktop's latest usage reading for one organization.
// Desktop records percentages only, no reset times.
type Sample struct {
	T        int64 // Unix seconds
	FiveHour float64
	SevenDay float64
}

// ReadDesktopSamples reads Desktop's plan-usage-history.json and keeps the
// newest sample per organization.
func ReadDesktopSamples(file string) (map[string]Sample, error) {
	out := map[string]Sample{}
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Samples []struct {
			T   int64  `json:"t"`
			Org string `json:"org"`
			U   struct {
				FH float64 `json:"fh"`
				SD float64 `json:"sd"`
			} `json:"u"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	for _, s := range doc.Samples {
		t := s.T / 1000
		if cur, ok := out[s.Org]; !ok || t > cur.T {
			out[s.Org] = Sample{T: t, FiveHour: s.U.FH, SevenDay: s.U.SD}
		}
	}
	return out, nil
}

// MergeDesktop overlays a Desktop sample on the record where it is newer.
// The statusLine's reset time is kept when it is still ahead of the sample.
func MergeDesktop(r Record, s *Sample) Record {
	if s == nil {
		return r
	}
	merge := func(w *Window, pct float64) *Window {
		if w != nil && s.T <= w.ObservedAt {
			return w
		}
		nw := &Window{UsedPercent: pct, ObservedAt: s.T, Source: "desktop"}
		if w != nil && w.ResetsAt > s.T {
			nw.ResetsAt = w.ResetsAt
		}
		return nw
	}
	r.FiveHour = merge(r.FiveHour, s.FiveHour)
	r.SevenDay = merge(r.SevenDay, s.SevenDay)
	return r
}
