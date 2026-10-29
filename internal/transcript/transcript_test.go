package transcript_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/transcript"
)

const observed = "You've hit your weekly limit · resets Nov 1 at 6pm (Asia/Shanghai)"

func shanghai(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestParseResetObservedFormat(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 28, 18, 0, 0, 0, loc)
	got, ok := transcript.ParseResetTime(observed, now)
	want := time.Date(2026, 11, 1, 18, 0, 0, 0, loc)
	if !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v", got, ok, want)
	}
}

func TestParseResetTimeOnlyRollsForward(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 28, 19, 0, 0, 0, loc)
	got, ok := transcript.ParseResetTime("You've hit your session limit · resets 6pm (Asia/Shanghai)", now)
	if want := time.Date(2026, 10, 29, 18, 0, 0, 0, loc); !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v", got, ok, want)
	}
	got, ok = transcript.ParseResetTime("limit reached · resets 9:30pm (Asia/Shanghai)", now)
	if want := time.Date(2026, 10, 28, 21, 30, 0, 0, loc); !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v", got, ok, want)
	}
	got, ok = transcript.ParseResetTime("limit reached · resets 23:15 (Asia/Shanghai)", now)
	if want := time.Date(2026, 10, 28, 23, 15, 0, 0, loc); !ok || !got.Equal(want) {
		t.Errorf("24h: got %v %v, want %v", got, ok, want)
	}
}

func TestParseResetDateRollsToNextYear(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 12, 30, 12, 0, 0, 0, loc)
	got, ok := transcript.ParseResetTime("limit · resets Jan 2 at 6am (Asia/Shanghai)", now)
	if want := time.Date(2027, 1, 2, 6, 0, 0, 0, loc); !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v", got, ok, want)
	}
}

func TestParseResetUnknownZone(t *testing.T) {
	if _, ok := transcript.ParseResetTime("limit · resets 6pm (Mars/Olympus)", time.Now()); ok {
		t.Error("unknown zone accepted")
	}
	if _, ok := transcript.ParseResetTime("no reset info here", time.Now()); ok {
		t.Error("garbage accepted")
	}
}

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(f, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestLastLimitMessageFindsLast(t *testing.T) {
	f := writeTranscript(t,
		`{"type":"assistant","isApiErrorMessage":true,"message":{"content":[{"type":"text","text":"old limit · resets 1pm"}]}}`,
		`{"type":"user","message":{"content":"hi"}}`,
		`{"type":"assistant","isApiErrorMessage":true,"message":{"content":[{"type":"text","text":"`+observed+`"}]}}`,
		`{"type":"system","subtype":"stop_hook_summary"}`,
	)
	got, ok := transcript.LastLimitMessage(f)
	if !ok || got != observed {
		t.Errorf("got %q %v", got, ok)
	}
}

func TestContextTokens(t *testing.T) {
	f := writeTranscript(t,
		`{"type":"assistant","message":{"usage":{"input_tokens":1,"cache_creation_input_tokens":10,"cache_read_input_tokens":100,"output_tokens":5}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":20000,"cache_read_input_tokens":160000,"output_tokens":7}}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"`+observed+`"}]}}`,
	)
	if got := transcript.ContextTokens(f); got != 180002 {
		t.Errorf("got %d", got)
	}
}

func TestTranscriptMissingFile(t *testing.T) {
	if _, ok := transcript.LastLimitMessage("/nonexistent/x.jsonl"); ok {
		t.Error("missing file returned a message")
	}
	if n := transcript.ContextTokens("/nonexistent/x.jsonl"); n != 0 {
		t.Errorf("got %d", n)
	}
}

// errorEntry is how Claude Code writes a limit notice: a synthetic assistant
// message with zero usage.
func errorEntry(text string) string {
	return `{"type":"assistant","isApiErrorMessage":true,"message":{"model":"<synthetic>","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"content":[{"type":"text","text":"` + text + `"}]}}`
}

func TestContextTokensSkipsSyntheticError(t *testing.T) {
	f := writeTranscript(t,
		`{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":20000,"cache_read_input_tokens":160000}}}`,
		errorEntry(observed),
	)
	if got := transcript.ContextTokens(f); got != 180002 {
		t.Errorf("got %d", got)
	}
}

func TestLastLimitMessageOnlyApiErrors(t *testing.T) {
	f := writeTranscript(t,
		errorEntry(observed),
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Your rate limit config resets 6pm daily."}]}}`,
	)
	if got, ok := transcript.LastLimitMessage(f); !ok || got != observed {
		t.Errorf("got %q %v", got, ok)
	}
}

func TestParseResetWithSuffix(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 28, 18, 0, 0, 0, loc)
	got, ok := transcript.ParseResetTime(observed+" · progress saved", now)
	if want := time.Date(2026, 11, 1, 18, 0, 0, 0, loc); !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v", got, ok, want)
	}
	got, ok = transcript.ParseResetTime("You've hit your session limit · resets 3pm · progress saved", time.Date(2026, 10, 29, 12, 0, 0, 0, time.Local))
	if !ok || got.Hour() != 15 {
		t.Errorf("time-only with suffix: %v %v", got, ok)
	}
}

func TestParseResetRejectsFarDates(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, loc)
	if got, ok := transcript.ParseResetTime("limit · resets Oct 25 at 6pm (Asia/Shanghai)", now); ok {
		t.Errorf("a date 4 days in the past was accepted as %v", got)
	}
	if got, ok := transcript.ParseResetTime("limit · resets Dec 25 at 6pm (Asia/Shanghai)", now); ok {
		t.Errorf("a date 2 months ahead was accepted as %v", got)
	}
}

func TestParseResetYearBoundaryPast(t *testing.T) {
	loc := shanghai(t)
	now := time.Date(2027, 1, 1, 1, 0, 0, 0, loc)
	got, ok := transcript.ParseResetTime("limit · resets Dec 31 at 11pm (Asia/Shanghai)", now)
	if want := time.Date(2026, 12, 31, 23, 0, 0, 0, loc); !ok || !got.Equal(want) {
		t.Errorf("got %v %v, want %v", got, ok, want)
	}
}
