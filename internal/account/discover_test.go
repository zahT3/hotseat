package account_test

import (
	"path/filepath"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/testutil"
)

func TestParseAliases(t *testing.T) {
	t.Setenv("HOME", "/Users/u")
	zshrc := `export PATH=/x:$PATH
alias claude2='CLAUDE_CONFIG_DIR="$HOME/.claude2" claude'
alias claude3="CLAUDE_CONFIG_DIR=${HOME}/.claude3 claude"
alias c4=CLAUDE_CONFIG_DIR=~/.claude4\ claude
alias work=CLAUDE_CONFIG_DIR=~/.claude-work\ claude
alias ll='ls -l'
`
	got := account.ParseAliases(zshrc)
	if got["/Users/u/.claude2"] != "claude2" {
		t.Errorf(".claude2 -> %q", got["/Users/u/.claude2"])
	}
	if got["/Users/u/.claude3"] != "claude3" {
		t.Errorf(".claude3 -> %q", got["/Users/u/.claude3"])
	}
	for dir := range got {
		if filepath.Base(dir) == "ll" {
			t.Errorf("non-claude alias parsed: %q", dir)
		}
	}
}

func TestDiscoverSkipsDirsWithoutIdentity(t *testing.T) {
	p := testutil.FakeHome(t)
	testutil.AddAccountDir(t, p, p.CoreDir(), "a1", "o1", nil)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claude2"), "b2", "o2", nil)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claude-deepseek"), "", "", nil)
	testutil.AddAccountDir(t, p, filepath.Join(p.Home, ".claudeX"), "c3", "o3", nil)
	testutil.WriteFile(t, p.Zshrc(), `alias claude2='CLAUDE_CONFIG_DIR="$HOME/.claude2" claude'`+"\n")

	got, err := account.Discover(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true, AccountUUID: "a1", OrgUUID: "o1"},
		{Name: "claude2", ConfigDir: filepath.Join(p.Home, ".claude2"), AccountUUID: "b2", OrgUUID: "o2"},
		{Name: "claudeX", ConfigDir: filepath.Join(p.Home, ".claudeX"), AccountUUID: "c3", OrgUUID: "o3"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d accounts: %+v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %+v, want %+v", i, got[i], want[i])
		}
	}
}
