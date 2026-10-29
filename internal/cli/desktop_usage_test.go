package cli_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/usage"
)

func desktopSessionEnv(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "claude-desktop")
}

func TestStatuslineAttributesDesktopToActive(t *testing.T) {
	p := desktopSetup(t)
	desktopSessionEnv(t)
	_, out := statusline(t, rlInput)
	if !strings.HasPrefix(strings.TrimSpace(out), "claude · 5h 16%") {
		t.Errorf("out %q", out)
	}
	if r, _ := usage.Load(p, "claude"); r.FiveHour == nil {
		t.Error("Desktop usage not recorded under the active Desktop account")
	}
}

func TestStatuslineDesktopUnknownWhenMismatch(t *testing.T) {
	p := desktopSetup(t)
	writeDesktopDir(t, p.DesktopDataDir(), "zz")
	desktopSessionEnv(t)
	_, out := statusline(t, rlInput)
	if strings.TrimSpace(out) != "" {
		t.Errorf("out %q", out)
	}
	if entries, _ := os.ReadDir(p.UsageDir()); len(entries) != 0 {
		t.Errorf("recorded despite identity mismatch: %v", entries)
	}
}

func TestStopFailureDesktopOffersDesktopSwitch(t *testing.T) {
	useSetup(t)
	desktopSessionEnv(t)
	in := fmt.Sprintf(`{"session_id":"s1","transcript_path":%q,"cwd":"/tmp/p"}`, limitTranscript(t))
	_, _, args, called := stopFailure(t, in)
	if !called || flag(args, "--from") != "claude" || flag(args, "--to") != "claude2" {
		t.Fatalf("called %v args %v", called, args)
	}
	found := false
	for _, a := range args {
		if a == "--desktop" {
			found = true
		}
	}
	if !found {
		t.Errorf("handoff not marked as a Desktop session: %v", args)
	}
}

func TestStopFailureDesktopSkipsAccountsWithoutDesktopData(t *testing.T) {
	desktopSetup(t)
	desktopSessionEnv(t)
	in := fmt.Sprintf(`{"session_id":"s1","transcript_path":%q,"cwd":"/tmp/p"}`, limitTranscript(t))
	_, _, args, _ := stopFailure(t, in)
	if flag(args, "--to") != "" {
		t.Errorf("recommended %q, which has no Desktop data", flag(args, "--to"))
	}
}
