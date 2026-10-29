package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/testutil"
	"github.com/zahT3/hotseat/internal/usage"
)

const limitText = "You've hit your weekly limit · resets Nov 1 at 6pm (Asia/Shanghai)"

func limitTranscript(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "sess-1.jsonl")
	testutil.WriteFile(t, f, `{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":20000,"cache_read_input_tokens":160000}}}`+"\n"+
		`{"type":"assistant","isApiErrorMessage":true,"message":{"model":"<synthetic>","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[{"type":"text","text":"`+limitText+`"}]}}`+"\n")
	return f
}

func stopFailure(t *testing.T, input string) (int, string, []string, bool) {
	t.Helper()
	var got []string
	called := false
	defer cli.SetSpawnHandoff(func(args []string) error { got, called = args, true; return nil })()
	loc, _ := time.LoadLocation("Asia/Shanghai")
	defer cli.SetClock(func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, loc) })()
	defer setStdin(input)()
	code, out, _ := run(t, "hook", "stop-failure")
	return code, out, got, called
}

func flag(args []string, name string) string {
	i := slices.Index(args, name)
	if i < 0 || i+1 >= len(args) {
		return "<missing>"
	}
	return args[i+1]
}

func TestStopFailureRecordsLimitAndSpawns(t *testing.T) {
	p := statusSetup(t)
	in := fmt.Sprintf(`{"session_id":"sess-1","transcript_path":%q,"cwd":"/tmp/x y","hook_event_name":"StopFailure"}`, limitTranscript(t))
	code, out, args, called := stopFailure(t, in)
	if code != 0 || out != "" || !called {
		t.Fatalf("exit %d out %q called %v", code, out, called)
	}
	r, _ := usage.Load(p, "claude2")
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if r.Limit == nil || r.Limit.ResetsAt != time.Date(2026, 11, 1, 18, 0, 0, 0, loc).Unix() || r.Limit.SessionID != "sess-1" {
		t.Errorf("limit = %+v", r.Limit)
	}
	want := map[string]string{"--from": "claude2", "--to": "claude", "--session": "sess-1", "--cwd": "/tmp/x y", "--tokens": "180002", "--message": limitText}
	for k, v := range want {
		if flag(args, k) != v {
			t.Errorf("%s = %q, want %q (args %v)", k, flag(args, k), v, args)
		}
	}
}

func TestStopFailureDesktopDoesNothing(t *testing.T) {
	p := statusSetup(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "claude-desktop")
	in := fmt.Sprintf(`{"session_id":"s","transcript_path":%q,"cwd":"/tmp"}`, limitTranscript(t))
	code, _, _, called := stopFailure(t, in)
	if code != 0 || called {
		t.Errorf("exit %d called %v", code, called)
	}
	if entries, _ := os.ReadDir(p.UsageDir()); len(entries) != 0 {
		t.Errorf("desktop session recorded: %v", entries)
	}
}

func TestStopFailureNoAlternative(t *testing.T) {
	p := testutil.FakeHome(t)
	testutil.AddAccountDir(t, p, p.CoreDir(), "a1", "o1", nil)
	run(t, "init")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	in := fmt.Sprintf(`{"session_id":"s","transcript_path":%q,"cwd":"/tmp"}`, limitTranscript(t))
	_, _, args, called := stopFailure(t, in)
	if !called || flag(args, "--to") != "" || flag(args, "--from") != "claude" {
		t.Errorf("called %v args %v", called, args)
	}
}

func TestStopFailureBadInput(t *testing.T) {
	statusSetup(t)
	code, out, _, called := stopFailure(t, "garbage")
	if code != 0 || out != "" || called {
		t.Errorf("exit %d out %q called %v", code, out, called)
	}
}

func TestStopFailurePrefersLastAssistantMessage(t *testing.T) {
	p := statusSetup(t)
	in := fmt.Sprintf(`{"session_id":"s","transcript_path":%q,"cwd":"/tmp","error":"rate_limit","last_assistant_message":"You've hit your session limit · resets 3pm (Asia/Shanghai) · progress saved"}`, limitTranscript(t))
	stopFailure(t, in)
	r, _ := usage.Load(p, "claude2")
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if r.Limit == nil || r.Limit.ResetsAt != time.Date(2026, 10, 29, 15, 0, 0, 0, loc).Unix() {
		t.Errorf("limit = %+v", r.Limit)
	}
}

func TestStopFailureTransientIgnored(t *testing.T) {
	p := statusSetup(t)
	tr := filepath.Join(t.TempDir(), "s.jsonl")
	testutil.WriteFile(t, tr, `{"type":"user","message":{"content":"hi"}}`+"\n")
	in := fmt.Sprintf(`{"session_id":"s","transcript_path":%q,"cwd":"/tmp","error":"rate_limit","last_assistant_message":"API Error: 429 rate_limit_error, retrying"}`, tr)
	_, _, _, called := stopFailure(t, in)
	if called {
		t.Error("a transient 429 offered a handoff")
	}
	if entries, _ := os.ReadDir(p.UsageDir()); len(entries) != 0 {
		t.Errorf("transient 429 recorded: %v", entries)
	}
}

func TestStopFailureRecommendUsesDesktop(t *testing.T) {
	p := statusSetup(t)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claude3"), "c3", "o3", nil)
	run(t, "init")
	old := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC).Unix()
	usage.Save(p, usage.Record{Account: "claude", SevenDay: &usage.Window{UsedPercent: 50, ResetsAt: 1_900_000_000, ObservedAt: old}})
	usage.Save(p, usage.Record{Account: "claude3", SevenDay: &usage.Window{UsedPercent: 20, ResetsAt: 1_900_000_000, ObservedAt: old}})
	testutil.WriteFile(t, filepath.Join(p.DesktopDataDir(), "plan-usage-history.json"),
		`{"version":1,"samples":[{"t":`+itoa((old+86400)*1000)+`,"org":"o3","u":{"fh":0,"sd":95}}]}`)
	in := fmt.Sprintf(`{"session_id":"s","transcript_path":%q,"cwd":"/tmp"}`, limitTranscript(t))
	_, _, args, _ := stopFailure(t, in)
	if flag(args, "--to") != "claude" {
		t.Errorf("--to = %q, want claude (claude3 is at 95%% per Desktop)", flag(args, "--to"))
	}
}
