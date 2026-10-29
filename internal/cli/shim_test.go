package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/testutil"
)

const zshrcOriginal = "export A=1\nalias claude2='CLAUDE_CONFIG_DIR=\"$HOME/.claude2\" claude'\n"

func TestShimInstallIdempotent(t *testing.T) {
	p := testutil.FakeHome(t)
	testutil.WriteFile(t, p.Zshrc(), zshrcOriginal)
	for i := 0; i < 2; i++ {
		if code, _, stderr := run(t, "shim", "install"); code != 0 {
			t.Fatalf("install #%d: %s", i+1, stderr)
		}
	}
	b, _ := os.ReadFile(p.Zshrc())
	got := string(b)
	if strings.Count(got, "# >>> hotseat >>>") != 1 || strings.Count(got, "# <<< hotseat <<<") != 1 {
		t.Errorf("want exactly one hotseat block:\n%s", got)
	}
	if !strings.HasPrefix(got, zshrcOriginal) {
		t.Errorf("original content changed:\n%s", got)
	}
	if !strings.Contains(got, `export PATH="$HOME/.config/hotseat/bin:$PATH"`) {
		t.Errorf("PATH line missing:\n%s", got)
	}
	self, _ := os.Executable()
	selfResolved, _ := filepath.EvalSymlinks(self)
	target, err := filepath.EvalSymlinks(filepath.Join(p.ShimDir(), "claude"))
	if err != nil || target != selfResolved {
		t.Errorf("shim -> %q (%v), want %q", target, err, selfResolved)
	}
	backups, _ := filepath.Glob(filepath.Join(p.BackupsDir(), "zshrc-*"))
	if len(backups) == 0 {
		t.Error("zshrc was not backed up")
	}
}

func TestShimUninstallRestores(t *testing.T) {
	p := testutil.FakeHome(t)
	testutil.WriteFile(t, p.Zshrc(), zshrcOriginal)
	run(t, "shim", "install")
	if code, _, stderr := run(t, "shim", "uninstall"); code != 0 {
		t.Fatalf("uninstall: %s", stderr)
	}
	b, _ := os.ReadFile(p.Zshrc())
	if string(b) != zshrcOriginal {
		t.Errorf("zshrc not restored:\n%q", b)
	}
	if _, err := os.Lstat(filepath.Join(p.ShimDir(), "claude")); !os.IsNotExist(err) {
		t.Errorf("shim symlink still present: %v", err)
	}
}

func TestShimInstallKeepsSymlinkedZshrc(t *testing.T) {
	p := testutil.FakeHome(t)
	real := filepath.Join(p.Home, "dotfiles", "zshrc")
	testutil.WriteFile(t, real, zshrcOriginal)
	if err := os.Symlink(real, p.Zshrc()); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(t, "shim", "install"); code != 0 {
		t.Fatalf("install: %s", stderr)
	}
	info, err := os.Lstat(p.Zshrc())
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("~/.zshrc symlink was replaced by a file")
	}
	b, _ := os.ReadFile(real)
	if !strings.Contains(string(b), "# >>> hotseat >>>") {
		t.Error("block not written through the symlink")
	}
}
