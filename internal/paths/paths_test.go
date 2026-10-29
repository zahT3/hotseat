package paths_test

import (
	"testing"

	"github.com/zahT3/hotseat/internal/paths"
)

func TestFromEnvDefaultsStateDir(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("HOTSEAT_HOME", "")
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"StateDir":          {p.StateDir, "/tmp/h/.config/hotseat"},
		"CoreDir":           {p.CoreDir(), "/tmp/h/.claude"},
		"DefaultClaudeJSON": {p.DefaultClaudeJSON(), "/tmp/h/.claude.json"},
		"RegistryFile":      {p.RegistryFile(), "/tmp/h/.config/hotseat/accounts.json"},
		"CurrentFile":       {p.CurrentFile(), "/tmp/h/.config/hotseat/current"},
		"OverridesFile":     {p.OverridesFile(), "/tmp/h/.config/hotseat/share-overrides.json"},
		"BackupsDir":        {p.BackupsDir(), "/tmp/h/.config/hotseat/backups"},
		"ShimDir":           {p.ShimDir(), "/tmp/h/.config/hotseat/bin"},
		"Zshrc":             {p.Zshrc(), "/tmp/h/.zshrc"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

func TestFromEnvHotseatHomeOverride(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	t.Setenv("HOTSEAT_HOME", "/tmp/r")
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if p.StateDir != "/tmp/r" {
		t.Errorf("StateDir = %q, want /tmp/r", p.StateDir)
	}
}

func TestFromEnvRequiresHome(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := paths.FromEnv(); err == nil {
		t.Fatal("expected error when HOME is empty")
	}
}
