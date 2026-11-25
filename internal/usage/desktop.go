package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Sample is Claude Desktop's latest usage reading for one organization.
// Desktop records percentages only, no reset times.
type Sample struct {
	T        int64    // Unix seconds
	FiveHour *float64 // nil when Desktop did not record it
	SevenDay *float64
}

// Fresh is the sample without the windows it is too old to describe.
// Desktop records no reset times, so age is the only evidence: a 5-hour
// reading older than five hours, or a weekly one older than seven days,
// belongs to a window that has ended. A parked profile's history stops
// when Desktop leaves it, so its last reading would otherwise stand forever.
func (s Sample) Fresh(now time.Time) Sample {
	age := now.Sub(time.Unix(s.T, 0))
	if age > 5*time.Hour {
		s.FiveHour = nil
	}
	if age > 7*24*time.Hour {
		s.SevenDay = nil
	}
	return s
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
				FH *float64 `json:"fh"`
				SD *float64 `json:"sd"`
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
	// A window Desktop did not record stays as it was: Desktop often
	// writes only the weekly number, and a missing one is not 0%.
	merge := func(w *Window, pct *float64) *Window {
		if pct == nil || (w != nil && s.T <= w.ObservedAt) {
			return w
		}
		nw := &Window{UsedPercent: *pct, ObservedAt: s.T, Source: "desktop"}
		if w != nil && w.ResetsAt > s.T {
			nw.ResetsAt = w.ResetsAt
		}
		return nw
	}
	r.FiveHour = merge(r.FiveHour, s.FiveHour)
	r.SevenDay = merge(r.SevenDay, s.SevenDay)
	return r
}
