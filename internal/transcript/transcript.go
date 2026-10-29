// Package transcript reads what Hotseat needs from a Claude Code session
// transcript (~/.claude/projects/<project>/<session>.jsonl): the last
// usage-limit message and how much context the session carries.
package transcript

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const tailBytes = 256 << 10

type entry struct {
	Type              string `json:"type"`
	IsAPIErrorMessage bool   `json:"isApiErrorMessage"`
	Message           struct {
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			Input         int `json:"input_tokens"`
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// tail returns the complete lines in the last tailBytes of the file, newest
// last.
func tail(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	off := info.Size() - tailBytes
	if off < 0 {
		off = 0
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	if off > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	return strings.Split(string(b), "\n")
}

func text(content json.RawMessage) string {
	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// IsLimitText reports whether s reads like a usage-limit notice rather than
// a transient error such as a 429 retry.
func IsLimitText(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "limit") && (strings.Contains(l, "resets") || strings.Contains(l, "hit your"))
}

// LastLimitMessage returns the text of the newest API-error entry that
// reads like a usage-limit notice. Ordinary assistant prose is ignored, so a
// model talking about limits never counts.
func LastLimitMessage(path string) (string, bool) {
	lines := tail(path)
	for i := len(lines) - 1; i >= 0; i-- {
		var e entry
		if json.Unmarshal([]byte(lines[i]), &e) != nil || e.Type != "assistant" || !e.IsAPIErrorMessage {
			continue
		}
		t := strings.TrimSpace(text(e.Message.Content))
		if IsLimitText(t) {
			return t, true
		}
	}
	return "", false
}

// ContextTokens is the input size of the session's latest real request:
// what a different account has to read again without cache when it
// continues. Claude Code's synthetic error entries carry zero usage and are
// skipped.
func ContextTokens(path string) int {
	lines := tail(path)
	for i := len(lines) - 1; i >= 0; i-- {
		var e entry
		if json.Unmarshal([]byte(lines[i]), &e) != nil || e.Type != "assistant" || e.Message.Usage == nil {
			continue
		}
		if e.IsAPIErrorMessage || e.Message.Model == "<synthetic>" {
			continue
		}
		u := e.Message.Usage
		if n := u.Input + u.CacheCreation + u.CacheRead; n > 0 {
			return n
		}
	}
	return 0
}

var (
	// "resets <when> [(<zone>)]", ending at " · ", a newline or the end, so
	// suffixes such as " · progress saved" do not break it.
	resetRe = regexp.MustCompile(`(?i)resets\s+(?:at\s+)?([^·()\n]+?)\s*(?:\(([^)]+)\))?\s*(?:·|\n|$)`)
	dateRe  = regexp.MustCompile(`(?i)^([A-Za-z]{3})[a-z]*\.?\s+(\d{1,2})(?:,|\s+at)?\s+(.+)$`)
	clockRe = regexp.MustCompile(`(?i)^(\d{1,2})(?::(\d{2}))?\s*(am|pm)?$`)
)

// ParseResetTime reads the reset moment from a limit message such as
// "You've hit your weekly limit · resets Nov 1 at 6pm (Asia/Shanghai)".
// Without a date it picks the next such time after now; without a zone it
// uses the local one.
func ParseResetTime(msg string, now time.Time) (time.Time, bool) {
	m := resetRe.FindStringSubmatch(msg)
	if m == nil {
		return time.Time{}, false
	}
	loc := time.Local
	if m[2] != "" {
		l, err := time.LoadLocation(strings.TrimSpace(m[2]))
		if err != nil {
			return time.Time{}, false
		}
		loc = l
	}
	now = now.In(loc)
	when := strings.TrimSpace(m[1])

	month, day := time.Month(0), 0
	if d := dateRe.FindStringSubmatch(when); d != nil {
		abbr := strings.ToUpper(d[1][:1]) + strings.ToLower(d[1][1:])
		mon, err := time.Parse("Jan", abbr)
		if err != nil {
			return time.Time{}, false
		}
		month = mon.Month()
		day, _ = strconv.Atoi(d[2])
		when = strings.TrimSpace(d[3])
	}
	c := clockRe.FindStringSubmatch(when)
	if c == nil {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(c[1])
	minute := 0
	if c[2] != "" {
		minute, _ = strconv.Atoi(c[2])
	}
	switch strings.ToLower(c[3]) {
	case "pm":
		if hour < 12 {
			hour += 12
		}
	case "am":
		if hour == 12 {
			hour = 0
		}
	}
	if hour > 23 || minute > 59 {
		return time.Time{}, false
	}

	if month == 0 {
		t := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		if !t.After(now) {
			t = t.AddDate(0, 0, 1)
		}
		return t, true
	}
	// The message has no year: take the candidate nearest to now, and accept
	// it only if it is a plausible reset (at most a day late, at most a week
	// and a day ahead).
	var t time.Time
	for _, y := range []int{now.Year() - 1, now.Year(), now.Year() + 1} {
		c := time.Date(y, month, day, hour, minute, 0, 0, loc)
		if t.IsZero() || absDur(c.Sub(now)) < absDur(t.Sub(now)) {
			t = c
		}
	}
	if t.Before(now.Add(-24*time.Hour)) || t.After(now.Add(8*24*time.Hour)) {
		return time.Time{}, false
	}
	return t, true
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
