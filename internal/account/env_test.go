package account_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
)

func envRegistry(t *testing.T) (account.Registry, string) {
	t.Helper()
	home := t.TempDir()
	for _, d := range []string{".claude", ".claude2"} {
		os.MkdirAll(filepath.Join(home, d), 0o755)
	}
	return account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: filepath.Join(home, ".claude"), IsDefault: true},
		{Name: "claude2", ConfigDir: filepath.Join(home, ".claude2")},
	}}, home
}

func TestFromEnvExplicitDir(t *testing.T) {
	reg, home := envRegistry(t)
	a, ok := account.FromEnv(reg, []string{"CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude2") + "/"})
	if !ok || a.Name != "claude2" {
		t.Errorf("got %+v %v", a, ok)
	}
}

func TestFromEnvDefault(t *testing.T) {
	reg, _ := envRegistry(t)
	for _, env := range [][]string{{"PATH=/bin"}, {"CLAUDE_CONFIG_DIR="}} {
		if a, ok := account.FromEnv(reg, env); !ok || a.Name != "claude" {
			t.Errorf("env %v: got %+v %v", env, a, ok)
		}
	}
}

func TestFromEnvDesktopIsUnknown(t *testing.T) {
	reg, _ := envRegistry(t)
	if a, ok := account.FromEnv(reg, []string{"CLAUDE_CODE_ENTRYPOINT=claude-desktop"}); ok {
		t.Errorf("desktop session attributed to %+v", a)
	}
}

func TestFromEnvUnregisteredDir(t *testing.T) {
	reg, home := envRegistry(t)
	if _, ok := account.FromEnv(reg, []string{"CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude-deepseek")}); ok {
		t.Error("unregistered dir matched")
	}
}

func TestFromEnvSymlinkedDir(t *testing.T) {
	reg, home := envRegistry(t)
	link := filepath.Join(home, "alias2")
	os.Symlink(filepath.Join(home, ".claude2"), link)
	if a, ok := account.FromEnv(reg, []string{"CLAUDE_CONFIG_DIR=" + link}); !ok || a.Name != "claude2" {
		t.Errorf("got %+v %v", a, ok)
	}
}
