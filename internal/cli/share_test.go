package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/testutil"
)

func TestApplyConflictLinePointsToAccountCopy(t *testing.T) {
	p := twoAccounts(t)
	testutil.WriteFile(t, filepath.Join(p.CoreDir(), "skills", "x", "SKILL.md"), "core")
	testutil.WriteFile(t, filepath.Join(p.Home, ".claude2", "skills", "x", "SKILL.md"), "acct")
	run(t, "init")
	code, out, stderr := run(t, "share", "apply", "--force")
	if code != 0 {
		t.Fatalf("apply: %s", stderr)
	}
	want := filepath.Join("claude2", "skills", "x", "SKILL.md")
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "account copy") && strings.Contains(line, "backups") && strings.Contains(line, want) {
			return
		}
	}
	t.Errorf("no line points at the account's copy in the backup:\n%s", out)
}
