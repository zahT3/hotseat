package shim

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/testutil"
)

func testRegistry(home string) account.Registry {
	return account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: filepath.Join(home, ".claude"), IsDefault: true, AccountUUID: "a1"},
		{Name: "claude2", ConfigDir: filepath.Join(home, ".claude2"), AccountUUID: "b2"},
	}}
}

func countPrefix(env []string, prefix string) int {
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			n++
		}
	}
	return n
}

func TestResolveEnvDefaultAccountUnsets(t *testing.T) {
	env := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR="}
	out, warn := ResolveEnv(env, testRegistry("/h"), "claude")
	if n := countPrefix(out, "CLAUDE_CONFIG_DIR="); n != 0 {
		t.Errorf("default account got CLAUDE_CONFIG_DIR (%d entries): %v", n, out)
	}
	if warn != "" {
		t.Errorf("unexpected warning %q", warn)
	}
}

func TestResolveEnvSetsNonDefault(t *testing.T) {
	env := []string{"PATH=/bin"}
	out, _ := ResolveEnv(env, testRegistry("/h"), "claude2")
	if n := countPrefix(out, "CLAUDE_CONFIG_DIR="); n != 1 {
		t.Fatalf("want exactly one CLAUDE_CONFIG_DIR, got %d: %v", n, out)
	}
	found := false
	for _, kv := range out {
		if kv == "CLAUDE_CONFIG_DIR=/h/.claude2" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing CLAUDE_CONFIG_DIR=/h/.claude2 in %v", out)
	}
}

func TestResolveEnvRespectsExplicitDir(t *testing.T) {
	env := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/x"}
	out, warn := ResolveEnv(env, testRegistry("/h"), "claude2")
	if !reflect.DeepEqual(out, env) || warn != "" {
		t.Errorf("got %v, %q", out, warn)
	}
}

func TestResolveEnvNoCurrentUnchanged(t *testing.T) {
	env := []string{"PATH=/bin"}
	out, warn := ResolveEnv(env, testRegistry("/h"), "")
	if !reflect.DeepEqual(out, env) || warn != "" {
		t.Errorf("got %v, %q", out, warn)
	}
}

func TestResolveEnvUnknownCurrentWarns(t *testing.T) {
	env := []string{"PATH=/bin"}
	out, warn := ResolveEnv(env, testRegistry("/h"), "gone")
	if !reflect.DeepEqual(out, env) {
		t.Errorf("env changed: %v", out)
	}
	if !strings.Contains(warn, `"gone" not found`) {
		t.Errorf("warning = %q", warn)
	}
}

func writeExec(t *testing.T, path string) {
	t.Helper()
	testutil.WriteFile(t, path, "#!/bin/sh\necho real\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFindRealClaudeSkipsShimAndSelf(t *testing.T) {
	base := t.TempDir()
	self := filepath.Join(base, "roster")
	writeExec(t, self)
	shimDir := filepath.Join(base, "shim")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(shimDir, "claude")); err != nil {
		t.Fatal(err)
	}
	linkToShim := filepath.Join(base, "link-to-shim")
	if err := os.Symlink(shimDir, linkToShim); err != nil {
		t.Fatal(err)
	}
	// A second symlink to roster outside the shim dir must also be skipped.
	otherDir := filepath.Join(base, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(otherDir, "claude")); err != nil {
		t.Fatal(err)
	}
	realDir := filepath.Join(base, "real")
	writeExec(t, filepath.Join(realDir, "claude"))

	pathEnv := strings.Join([]string{shimDir, linkToShim, otherDir, realDir, realDir}, ":")
	got, err := FindRealClaude(pathEnv, self, shimDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(realDir, "claude") {
		t.Errorf("got %q", got)
	}
}

func TestFindRealClaudeNone(t *testing.T) {
	base := t.TempDir()
	if _, err := FindRealClaude(base, filepath.Join(base, "self"), filepath.Join(base, "shim")); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunFallsBackOnCorruptRegistry(t *testing.T) {
	p := testutil.FakeHome(t)
	testutil.WriteFile(t, p.RegistryFile(), "{")
	testutil.WriteFile(t, p.CurrentFile(), "claude2\n")
	realDir := t.TempDir()
	writeExec(t, filepath.Join(realDir, "claude"))
	t.Setenv("PATH", realDir)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	var gotBin string
	var gotArgs, gotEnv []string
	origExec, origStderr := execFn, stderr
	t.Cleanup(func() { execFn, stderr = origExec, origStderr })
	execFn = func(bin string, args, env []string) error {
		gotBin, gotArgs, gotEnv = bin, args, env
		return nil
	}
	var errb bytes.Buffer
	stderr = &errb

	before := os.Environ()
	Run([]string{"claude", "--version"})

	if gotBin != filepath.Join(realDir, "claude") {
		t.Errorf("exec bin = %q", gotBin)
	}
	if !reflect.DeepEqual(gotArgs, []string{"claude", "--version"}) {
		t.Errorf("args = %v", gotArgs)
	}
	if !reflect.DeepEqual(gotEnv, before) {
		t.Errorf("env changed despite corrupt registry")
	}
	if !strings.HasPrefix(errb.String(), "roster:") {
		t.Errorf("stderr = %q", errb.String())
	}
}
