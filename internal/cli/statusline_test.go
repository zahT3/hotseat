package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/cli"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
	"github.com/zahT3/hotseat/internal/usage"
)

// statusSetup registers claude and claude2 and runs as claude2.
func statusSetup(t *testing.T) paths.Paths {
	t.Helper()
	p := twoAccounts(t)
	run(t, "init")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(p.Home, ".claude2"))
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "cli")
	return p
}

func setStdin(input string) func() { return cli.SetStdin(strings.NewReader(input)) }

func statusline(t *testing.T, input string) (int, string) {
	t.Helper()
	restore := cli.SetStdin(strings.NewReader(input))
	defer restore()
	code, out, _ := run(t, "statusline")
	return code, out
}

const rlInput = `{"rate_limits":{"five_hour":{"used_percentage":16.4,"resets_at":2000000000},"seven_day":{"used_percentage":12,"resets_at":2000300000}}}`

func TestStatuslineRecordsAndPrints(t *testing.T) {
	p := statusSetup(t)
	code, out := statusline(t, rlInput)
	if code != 0 || strings.TrimSpace(out) != "claude2 · 5h 16% · 7d 12%" {
		t.Errorf("exit %d, out %q", code, out)
	}
	r, _ := usage.Load(p, "claude2")
	if r.FiveHour == nil || r.FiveHour.ResetsAt != 2000000000 {
		t.Errorf("not recorded: %+v", r)
	}
}

func TestStatuslineGarbageInput(t *testing.T) {
	statusSetup(t)
	code, out := statusline(t, "not json")
	if code != 0 || strings.TrimSpace(out) != "claude2" {
		t.Errorf("exit %d, out %q", code, out)
	}
}

func TestStatuslineDesktopRecordsNothing(t *testing.T) {
	p := statusSetup(t)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "claude-desktop")
	code, out := statusline(t, rlInput)
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("exit %d, out %q", code, out)
	}
	if entries, _ := os.ReadDir(p.UsageDir()); len(entries) != 0 {
		t.Errorf("desktop session recorded usage: %v", entries)
	}
}

func TestStatuslineChainsPrevious(t *testing.T) {
	p := statusSetup(t)
	testutil.WriteFile(t, p.ChainFile(), `{"command":"cat >/dev/null; echo prev; echo second"}`)
	_, out := statusline(t, rlInput)
	if !strings.HasPrefix(out, "prev | claude2") {
		t.Errorf("out %q", out)
	}
}

func TestStatuslineChainTimeout(t *testing.T) {
	p := statusSetup(t)
	testutil.WriteFile(t, p.ChainFile(), `{"command":"sleep 5"}`)
	start := time.Now()
	code, out := statusline(t, rlInput)
	if time.Since(start) > time.Second || code != 0 || !strings.Contains(out, "claude2") {
		t.Errorf("took %v, exit %d, out %q", time.Since(start), code, out)
	}
}

func TestStatuslineCorruptRegistry(t *testing.T) {
	p := statusSetup(t)
	testutil.WriteFile(t, p.RegistryFile(), "{")
	code, out := statusline(t, rlInput)
	if code != 0 || strings.TrimSpace(out) != "" {
		t.Errorf("exit %d, out %q", code, out)
	}
}
