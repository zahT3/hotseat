package share_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/share"
	"github.com/zahT3/hotseat/internal/testutil"
)

// An account dir that already linked single skills into ~/.claude/skills is
// the same content, not a conflict.
func TestMergeDirSymlinkToSameEntryIsNotConflict(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("skills/x/SKILL.md"), "x")
	symlink(t, f.core("skills/x"), f.acctPath("skills/x"))
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range m.Steps {
		if len(s.Conflicts) > 0 {
			t.Errorf("false conflict: %v", s.Conflicts)
		}
	}
}

func TestPlanCoreLinksIntoAccount_Conflict(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.acctPath("plans/p.md"), "p")
	symlink(t, f.acctPath("plans"), f.core("plans"))
	expectKind(t, f.plan(t), "plans", share.Conflict)
}

func TestApplyNothingToDoCreatesNoBackup(t *testing.T) {
	f := newFixture(t)
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "" {
		t.Errorf("empty plan produced backup %q", m.ID)
	}
	if ids, _ := share.ListBackups(f.p); len(ids) != 0 {
		t.Errorf("backups = %v", ids)
	}
}

func TestRollbackRefusesOlderWhileNewerApplied(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("history.jsonl"), `{"timestamp":1}`+"\n")
	testutil.WriteFile(t, f.acctPath("history.jsonl"), `{"timestamp":2}`+"\n")
	older, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	mkdir(t, f.core("plans"))
	newer, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil || newer.ID == "" || newer.ID == older.ID {
		t.Fatalf("second apply: %+v %v", newer, err)
	}
	_, err = share.Rollback(f.p, older.ID, share.RollbackOptions{Running: noClaude})
	if err == nil || !strings.Contains(err.Error(), newer.ID) {
		t.Errorf("rolling back %s while %s is applied: err = %v", older.ID, newer.ID, err)
	}
}

func TestMergeJSONLKeepsSymlinkedCoreHistory(t *testing.T) {
	f := newFixture(t)
	real := filepath.Join(f.p.Home, "sync", "history.jsonl")
	testutil.WriteFile(t, real, `{"timestamp":1}`+"\n")
	symlink(t, real, f.core("history.jsonl"))
	testutil.WriteFile(t, f.acctPath("history.jsonl"), `{"timestamp":2}`+"\n")
	if _, err := f.apply(t, share.ApplyOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(f.core("history.jsonl"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("core history.jsonl is no longer a symlink")
	}
	b, _ := os.ReadFile(real)
	if !strings.Contains(string(b), `"timestamp":2`) {
		t.Errorf("merge not written to the symlink target: %q", b)
	}
}
