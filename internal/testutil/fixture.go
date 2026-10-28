// Package testutil builds fake HOME layouts so tests never touch the real
// ~/.claude* directories.
package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/zahT3/roster/internal/paths"
)

// FakeHome points HOME and ROSTER_HOME at fresh temp dirs for the test.
func FakeHome(t *testing.T) paths.Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ROSTER_HOME", filepath.Join(home, ".config", "roster"))
	p, err := paths.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// AddAccountDir creates a Claude config dir with a .claude.json. An empty
// acctUUID writes a .claude.json without oauthAccount (an API-key or
// provider-bridge dir). For the core dir the file goes to $HOME/.claude.json,
// which is where Claude Code keeps it when CLAUDE_CONFIG_DIR is unset.
func AddAccountDir(t *testing.T, p paths.Paths, dir, acctUUID, orgUUID string, mcp map[string]any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"numStartups": 1}
	if acctUUID != "" {
		doc["oauthAccount"] = map[string]any{"accountUuid": acctUUID, "organizationUuid": orgUUID}
	}
	if mcp != nil {
		doc["mcpServers"] = mcp
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, ".claude.json")
	if filepath.Clean(dir) == p.CoreDir() {
		target = p.DefaultClaudeJSON()
	}
	if err := os.WriteFile(target, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// WriteFile writes content to path, creating parent dirs.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
