package paths_test

import (
	"testing"

	"github.com/zahT3/roster/internal/paths"
)

func TestFromEnvDefaultsRosterDir(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("ROSTER_HOME", "")
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"RosterDir":         {p.RosterDir, "/tmp/h/.config/roster"},
		"CoreDir":           {p.CoreDir(), "/tmp/h/.claude"},
		"DefaultClaudeJSON": {p.DefaultClaudeJSON(), "/tmp/h/.claude.json"},
		"RegistryFile":      {p.RegistryFile(), "/tmp/h/.config/roster/accounts.json"},
		"CurrentFile":       {p.CurrentFile(), "/tmp/h/.config/roster/current"},
		"OverridesFile":     {p.OverridesFile(), "/tmp/h/.config/roster/share-overrides.json"},
		"BackupsDir":        {p.BackupsDir(), "/tmp/h/.config/roster/backups"},
		"ShimDir":           {p.ShimDir(), "/tmp/h/.config/roster/bin"},
		"Zshrc":             {p.Zshrc(), "/tmp/h/.zshrc"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

func TestFromEnvRosterHomeOverride(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("ROSTER_HOME", "/tmp/r")
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if p.RosterDir != "/tmp/r" {
		t.Errorf("RosterDir = %q, want /tmp/r", p.RosterDir)
	}
}

func TestFromEnvRequiresHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := paths.FromEnv(); err == nil {
		t.Fatal("expected error when HOME is empty")
	}
}
