package share

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func writeClaudeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMergedMCPConflictKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, "default.json")
	other := filepath.Join(dir, "other.json")
	writeClaudeJSON(t, def, `{"mcpServers":{"a":{"command":"x"}}}`)
	writeClaudeJSON(t, other, `{"mcpServers":{"a":{"command":"y"},"b":{}}}`)

	merged, conflicts, err := mergedMCP([]string{def, other}, def)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !reflect.DeepEqual(keys, []string{"a", "b"}) {
		t.Errorf("keys = %v", keys)
	}
	var a map[string]string
	if err := json.Unmarshal(merged["a"], &a); err != nil || a["command"] != "x" {
		t.Errorf("a = %s", merged["a"])
	}
	if !reflect.DeepEqual(conflicts, []string{"a"}) {
		t.Errorf("conflicts = %v", conflicts)
	}
}

func TestMergedMCPSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	def := filepath.Join(dir, "default.json")
	writeClaudeJSON(t, def, `{"numStartups":1}`)
	merged, conflicts, err := mergedMCP([]string{def, filepath.Join(dir, "missing.json")}, def)
	if err != nil || len(merged) != 0 || len(conflicts) != 0 {
		t.Errorf("merged=%v conflicts=%v err=%v", merged, conflicts, err)
	}
}
