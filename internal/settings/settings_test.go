package settings_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/settings"
)

// Shaped like a hand-written ~/.claude/settings.json.
const fixture = `{
  "env": {
    "A": "1"
  },
  "permissions": {
    "allow": ["Bash(ls *)"],
    "defaultMode": "auto"
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "~/hooks/delete-guard.py"}]
      }
    ],
    "UserPromptSubmit": [
      {"hooks": [{"type": "command", "command": "~/hooks/prompt-time.sh"}]}
    ]
  },
  "enabledPlugins": {"x@y": true},
  "theme": "dark"
}
`

func setup(t *testing.T, content string) (file, backup string) {
	t.Helper()
	dir := t.TempDir()
	file = filepath.Join(dir, "settings.json")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return file, filepath.Join(dir, "backup.json")
}

func read(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func members(t *testing.T, b []byte) map[string]string {
	t.Helper()
	ms, err := settings.TopLevel(b)
	if err != nil {
		t.Fatalf("TopLevel: %v\n%s", err, b)
	}
	out := map[string]string{}
	for _, m := range ms {
		out[m.Key] = string(b[m.ValueStart:m.ValueEnd])
	}
	return out
}

func TestSetStatusLineKeepsOtherBytes(t *testing.T) {
	file, backup := setup(t, fixture)
	before := members(t, []byte(fixture))
	prev, err := settings.SetStatusLine(file, "hotseat statusline", backup)
	if err != nil || prev != nil {
		t.Fatalf("prev %s, err %v", prev, err)
	}
	after := members(t, read(t, file))
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s changed:\n%s\n---\n%s", k, v, after[k])
		}
	}
	var sl struct{ Type, Command string }
	json.Unmarshal([]byte(after["statusLine"]), &sl)
	if sl.Type != "command" || sl.Command != "hotseat statusline" {
		t.Errorf("statusLine = %s", after["statusLine"])
	}
	if string(read(t, backup)) != fixture {
		t.Error("backup is not the original file")
	}
}

func TestRestoreStatusLineRemovesKey(t *testing.T) {
	file, backup := setup(t, fixture)
	settings.SetStatusLine(file, "hotseat statusline", backup)
	if err := settings.RestoreStatusLine(file, nil, backup); err != nil {
		t.Fatal(err)
	}
	if got := string(read(t, file)); got != fixture {
		t.Errorf("not restored byte for byte:\n%s", got)
	}
}

func TestRestoreStatusLinePrevious(t *testing.T) {
	orig := strings.Replace(fixture, `"theme": "dark"`, `"statusLine": {"type": "command", "command": "echo prev", "padding": 1},
  "theme": "dark"`, 1)
	file, backup := setup(t, orig)
	prev, err := settings.SetStatusLine(file, "hotseat statusline", backup)
	if err != nil || !strings.Contains(string(prev), "echo prev") {
		t.Fatalf("prev %s, err %v", prev, err)
	}
	if err := settings.RestoreStatusLine(file, prev, backup); err != nil {
		t.Fatal(err)
	}
	if got := string(read(t, file)); got != orig {
		t.Errorf("not restored:\n%s", got)
	}
}

func hooksOf(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var doc struct{ Hooks map[string]any }
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b)
	}
	return doc.Hooks
}

func TestAddHookKeepsExisting(t *testing.T) {
	file, backup := setup(t, fixture)
	if err := settings.AddHook(file, "StopFailure", "rate_limit", "hotseat hook stop-failure", backup); err != nil {
		t.Fatal(err)
	}
	before, after := hooksOf(t, []byte(fixture)), hooksOf(t, read(t, file))
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit"} {
		if !reflect.DeepEqual(before[ev], after[ev]) {
			t.Errorf("%s changed", ev)
		}
	}
	sf, _ := json.Marshal(after["StopFailure"])
	if !strings.Contains(string(sf), `"matcher":"rate_limit"`) || !strings.Contains(string(sf), "hotseat hook stop-failure") {
		t.Errorf("StopFailure = %s", sf)
	}
	b := read(t, file)
	m := members(t, b)
	if m["env"] != members(t, []byte(fixture))["env"] {
		t.Error("other top-level bytes changed")
	}
}

func TestAddHookIdempotent(t *testing.T) {
	file, backup := setup(t, fixture)
	settings.AddHook(file, "StopFailure", "rate_limit", "hotseat hook stop-failure", backup)
	settings.AddHook(file, "StopFailure", "rate_limit", "hotseat hook stop-failure", backup)
	sf, _ := json.Marshal(hooksOf(t, read(t, file))["StopFailure"])
	if n := strings.Count(string(sf), "hotseat hook stop-failure"); n != 1 {
		t.Errorf("%d entries: %s", n, sf)
	}
}

func TestRemoveHookRemovesEmptyEvent(t *testing.T) {
	file, backup := setup(t, fixture)
	settings.AddHook(file, "StopFailure", "rate_limit", "hotseat hook stop-failure", backup)
	if err := settings.RemoveHook(file, "StopFailure", "hotseat hook stop-failure", backup); err != nil {
		t.Fatal(err)
	}
	h := hooksOf(t, read(t, file))
	if _, ok := h["StopFailure"]; ok {
		t.Errorf("empty StopFailure left: %v", h)
	}
	if !reflect.DeepEqual(h, hooksOf(t, []byte(fixture))) {
		t.Error("other hooks changed")
	}
}

func TestAddHookWithoutHooksKey(t *testing.T) {
	file, backup := setup(t, "{\n  \"theme\": \"dark\"\n}\n")
	if err := settings.AddHook(file, "StopFailure", "rate_limit", "hotseat hook stop-failure", backup); err != nil {
		t.Fatal(err)
	}
	if _, ok := hooksOf(t, read(t, file))["StopFailure"]; !ok {
		t.Error("hook not added")
	}
}

func TestWritesThroughSymlink(t *testing.T) {
	file, backup := setup(t, fixture)
	link := filepath.Join(filepath.Dir(file), "link.json")
	os.Symlink(file, link)
	if _, err := settings.SetStatusLine(link, "hotseat statusline", backup); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Lstat(link); info.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink replaced by a file")
	}
	if !strings.Contains(string(read(t, file)), "hotseat statusline") {
		t.Error("target not written")
	}
}
