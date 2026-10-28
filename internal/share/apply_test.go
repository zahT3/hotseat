package share_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zahT3/roster/internal/share"
	"github.com/zahT3/roster/internal/testutil"
)

func noClaude() ([]int, error) { return nil, nil }

// snapshot records every file's bytes, every symlink's target and every dir
// under the two config dirs, plus both .claude.json files.
func snapshot(t *testing.T, f fixture) map[string]string {
	t.Helper()
	snap := map[string]string{}
	for _, root := range []string{f.p.CoreDir(), f.acct} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				target, _ := os.Readlink(path)
				snap[path] = "link:" + target
			case info.IsDir():
				snap[path] = "dir"
			default:
				snap[path] = "file:" + readForSnap(path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, j := range []string{f.p.DefaultClaudeJSON(), filepath.Join(f.acct, ".claude.json")} {
		snap[j] = "file:" + readForSnap(j)
	}
	return snap
}

// readForSnap returns file content; .claude.json is compared by meaning
// because roster rewrites it when it changes the mcpServers key.
func readForSnap(path string) string {
	b, _ := os.ReadFile(path)
	if filepath.Base(path) != ".claude.json" {
		return string(b)
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return string(b)
	}
	canon, _ := json.Marshal(v)
	return string(canon)
}

func diffSnap(t *testing.T, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s changed:\n before %q\n after  %q", k, v, after[k])
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s appeared", k)
		}
	}
}

func richFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	testutil.AddAccountDir(t, f.p, f.p.CoreDir(), "a1", "o1", map[string]any{"a": map[string]any{"command": "x"}})
	testutil.AddAccountDir(t, f.p, f.acct, "b2", "o2", map[string]any{"b": map[string]any{"command": "y"}})
	testutil.WriteFile(t, f.core("history.jsonl"), `{"display":"core","timestamp":1}`+"\n")
	testutil.WriteFile(t, f.acctPath("history.jsonl"), `{"display":"acct","timestamp":2}`+"\n")
	testutil.WriteFile(t, f.core("skills/x/SKILL.md"), "core version")
	testutil.WriteFile(t, f.acctPath("skills/x/SKILL.md"), "acct version")
	testutil.WriteFile(t, f.acctPath("skills/y/SKILL.md"), "only in acct")
	testutil.WriteFile(t, f.core("file-history/s0/v1"), "core snap")
	testutil.WriteFile(t, f.acctPath("file-history/s1/v1"), "acct snap")
	mkdir(t, f.core("tasks"))
	return f
}

func (f fixture) apply(t *testing.T, opts share.ApplyOptions) (share.Manifest, error) {
	t.Helper()
	return share.Apply(f.p, f.reg, f.plan(t), opts)
}

func isLinkTo(t *testing.T, link, target string) bool {
	t.Helper()
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	got, _ := filepath.EvalSymlinks(link)
	want, _ := filepath.EvalSymlinks(target)
	return got == want
}

func TestApplyRefusesWhenClaudeRunning(t *testing.T) {
	f := richFixture(t)
	before := snapshot(t, f)
	_, err := f.apply(t, share.ApplyOptions{Running: func() ([]int, error) { return []int{123}, nil }})
	if !errors.Is(err, share.ErrClaudeRunning) {
		t.Fatalf("err = %v, want ErrClaudeRunning", err)
	}
	if !strings.Contains(err.Error(), "123") {
		t.Errorf("error should name the pid: %v", err)
	}
	diffSnap(t, before, snapshot(t, f))
}

func TestApplyThenRollbackRestoresBytes(t *testing.T) {
	f := richFixture(t)
	before := snapshot(t, f)

	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"history.jsonl", "skills", "file-history", "tasks"} {
		if !isLinkTo(t, f.acctPath(item), f.core(item)) {
			t.Errorf("%s is not a symlink to core", item)
		}
	}
	hist, _ := os.ReadFile(f.core("history.jsonl"))
	if !strings.Contains(string(hist), `"core"`) || !strings.Contains(string(hist), `"acct"`) {
		t.Errorf("merged history = %q", hist)
	}
	if b, _ := os.ReadFile(f.core("skills/x/SKILL.md")); string(b) != "core version" {
		t.Errorf("core skill overwritten: %q", b)
	}
	if b, _ := os.ReadFile(f.core("skills/y/SKILL.md")); string(b) != "only in acct" {
		t.Errorf("acct-only skill not copied: %q", b)
	}
	if b, _ := os.ReadFile(f.core("file-history/s1/v1")); string(b) != "acct snap" {
		t.Errorf("acct file-history not copied: %q", b)
	}
	var sawConflict bool
	for _, s := range m.Steps {
		for _, c := range s.Conflicts {
			if strings.HasSuffix(c, filepath.Join("skills", "x", "SKILL.md")) {
				sawConflict = true
			}
		}
	}
	if !sawConflict {
		t.Errorf("skills/x/SKILL.md conflict not recorded: %+v", m.Steps)
	}

	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	diffSnap(t, before, snapshot(t, f))
}

func TestRollbackTwiceFails(t *testing.T) {
	f := richFixture(t)
	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err == nil {
		t.Error("second rollback should fail")
	}
}

func TestRollbackAfterPartialFailure(t *testing.T) {
	f := newFixture(t)
	testutil.WriteFile(t, f.core("file-history/s0/v1"), "core")
	testutil.WriteFile(t, f.acctPath("file-history/s1/v1"), "acct")
	testutil.WriteFile(t, f.core("history.jsonl"), `{"timestamp":1}`+"\n")
	testutil.WriteFile(t, f.acctPath("history.jsonl"), `{"timestamp":2}`+"\n")
	before := snapshot(t, f)

	share.SetBeforeStep(t, func(i int) {
		if i == 1 {
			if err := os.Chmod(f.p.CoreDir(), 0o555); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Cleanup(func() { os.Chmod(f.p.CoreDir(), 0o755) })

	m, err := f.apply(t, share.ApplyOptions{Running: noClaude})
	if err == nil {
		t.Fatal("expected apply to fail on read-only core")
	}
	if err := os.Chmod(f.p.CoreDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// The failing step is logged as intent before it mutates anything, so the
	// manifest may hold it too; what matters is that rollback restores everything.
	if len(m.Steps) == 0 || m.Steps[0].Action.Item != "file-history" {
		t.Fatalf("manifest steps = %+v", m.Steps)
	}
	ids, err := share.ListBackups(f.p)
	if err != nil || len(ids) != 1 || ids[0] != m.ID {
		t.Fatalf("ListBackups = %v, %v (manifest %s)", ids, err, m.ID)
	}
	if _, err := share.Rollback(f.p, m.ID, share.RollbackOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	diffSnap(t, before, snapshot(t, f))
}

func TestSyncMCPPreservesOtherKeys(t *testing.T) {
	f := richFixture(t)
	read := func(path string) map[string]any {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	files := []string{f.p.DefaultClaudeJSON(), filepath.Join(f.acct, ".claude.json")}
	var before []map[string]any
	for _, file := range files {
		before = append(before, read(file))
	}
	if _, err := f.apply(t, share.ApplyOptions{Running: noClaude}); err != nil {
		t.Fatal(err)
	}
	for i, file := range files {
		after := read(file)
		servers, _ := after["mcpServers"].(map[string]any)
		if _, ok := servers["a"]; !ok {
			t.Errorf("%s missing server a: %v", file, servers)
		}
		if _, ok := servers["b"]; !ok {
			t.Errorf("%s missing server b: %v", file, servers)
		}
		delete(after, "mcpServers")
		delete(before[i], "mcpServers")
		if !reflect.DeepEqual(after, before[i]) {
			t.Errorf("%s other keys changed:\n before %v\n after  %v", file, before[i], after)
		}
	}
}
