// Package usage keeps the last known 5-hour and 7-day usage of each account.
// Data only comes from what Claude Code hands to the statusLine and hooks,
// from limit messages in transcripts, and from Claude Desktop's local usage
// history; nothing here queries Anthropic.
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zahT3/hotseat/internal/paths"
)

type Window struct {
	UsedPercent float64 `json:"usedPercent"`
	ResetsAt    int64   `json:"resetsAt"` // Unix seconds; 0 = unknown
	ObservedAt  int64   `json:"observedAt"`
	Source      string  `json:"source"`
}

type LimitHit struct {
	Message    string `json:"message"`
	ResetsAt   int64  `json:"resetsAt"` // 0 = unknown
	ObservedAt int64  `json:"observedAt"`
	SessionID  string `json:"sessionId,omitempty"`
	Cwd        string `json:"cwd,omitempty"`
}

type Record struct {
	Account  string    `json:"account"`
	FiveHour *Window   `json:"fiveHour,omitempty"`
	SevenDay *Window   `json:"sevenDay,omitempty"`
	Limit    *LimitHit `json:"limit,omitempty"`
}

// RateWindow and RateLimits mirror the rate_limits object of the statusLine
// JSON.
type RateWindow struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type RateLimits struct {
	FiveHour *RateWindow `json:"five_hour"`
	SevenDay *RateWindow `json:"seven_day"`
}

func file(p paths.Paths, name string) string {
	return filepath.Join(p.UsageDir(), name+".json")
}

func Load(p paths.Paths, name string) (Record, error) {
	b, err := os.ReadFile(file(p, name))
	if errors.Is(err, os.ErrNotExist) {
		return Record{Account: name}, nil
	}
	if err != nil {
		return Record{}, err
	}
	var r Record
	if err := json.Unmarshal(b, &r); err != nil {
		return Record{}, fmt.Errorf("parse %s: %w", file(p, name), err)
	}
	r.Account = name
	return r, nil
}

func Save(p paths.Paths, r Record) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.UsageDir(), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(p.UsageDir(), "."+r.Account+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file(p, r.Account))
}

// ObserveRateLimits records the windows present in rl. The statusLine runs
// on every render, so the file is only rewritten when a value changed.
func ObserveRateLimits(p paths.Paths, name string, rl RateLimits, now time.Time) error {
	r, err := Load(p, name)
	if err != nil {
		return err
	}
	changed := false
	apply := func(dst **Window, src *RateWindow) {
		if src == nil {
			return
		}
		if *dst != nil && (*dst).UsedPercent == src.UsedPercentage && (*dst).ResetsAt == src.ResetsAt {
			return
		}
		*dst = &Window{UsedPercent: src.UsedPercentage, ResetsAt: src.ResetsAt, ObservedAt: now.Unix(), Source: "statusline"}
		changed = true
	}
	apply(&r.FiveHour, rl.FiveHour)
	apply(&r.SevenDay, rl.SevenDay)
	if (rl.FiveHour != nil || rl.SevenDay != nil) && r.Limit != nil && r.Limit.ResetsAt > 0 && r.Limit.ResetsAt <= now.Unix() {
		r.Limit = nil
		changed = true
	}
	if !changed {
		return nil
	}
	return Save(p, r)
}

// Effective is the usage a window stands at now: a window whose reset time
// has passed is back to 0.
func Effective(w *Window, now time.Time) (pct float64, known, reset bool) {
	if w == nil {
		return 0, false, false
	}
	if w.ResetsAt > 0 && w.ResetsAt <= now.Unix() {
		return 0, true, true
	}
	return w.UsedPercent, true, false
}
