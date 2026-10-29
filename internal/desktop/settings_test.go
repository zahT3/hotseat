package desktop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/testutil"
)

func readJSON(t *testing.T, f string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read %s: %v", f, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse %s: %v", f, err)
	}
	return m
}

const cfg = "claude_desktop_config.json"

func TestSyncSettingsRemapsByAccount(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, cfg), `{"preferences":{"bypassPermissionsGateByAccount":{"a1":true,"zz":false},"sidebarMode":"x"}}`)
	testutil.WriteFile(t, filepath.Join(dst, cfg), `{"preferences":{"bypassPermissionsGateByAccount":{"b2":false,"c3":true},"dstOnly":1}}`)
	if err := desktop.SyncSettings(src, dst, "a1", "b2", bk); err != nil {
		t.Fatal(err)
	}
	prefs := readJSON(t, filepath.Join(dst, cfg))["preferences"].(map[string]any)
	if !reflect.DeepEqual(prefs["bypassPermissionsGateByAccount"], map[string]any{"b2": true, "c3": true}) {
		t.Errorf("ByAccount = %v", prefs["bypassPermissionsGateByAccount"])
	}
	if prefs["sidebarMode"] != "x" || prefs["dstOnly"] != float64(1) {
		t.Errorf("prefs = %v", prefs)
	}
}

func TestSyncSettingsCopiesMCP(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, cfg), `{"mcpServers":{"m":{"command":"x"}},"preferences":{}}`)
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	if m := readJSON(t, filepath.Join(dst, cfg))["mcpServers"].(map[string]any); m["m"] == nil {
		t.Errorf("mcpServers = %v", m)
	}
}

func TestSyncSettingsRemapsPresets(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "ccd-session-presets.json"), `{"byAccount":{"a1":{"model":"opus"}}}`)
	testutil.WriteFile(t, filepath.Join(dst, "ccd-session-presets.json"), `{"byAccount":{"c3":{"model":"haiku"}}}`)
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	by := readJSON(t, filepath.Join(dst, "ccd-session-presets.json"))["byAccount"].(map[string]any)
	if by["b2"] == nil || by["c3"] == nil || by["a1"] != nil {
		t.Errorf("byAccount = %v", by)
	}
}

func TestSyncSettingsMirrorsExtensionsWithBackup(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "Claude Extensions", "e1", "manifest.json"), "{}")
	testutil.WriteFile(t, filepath.Join(src, "extensions-installations.json"), `{"e1":{}}`)
	testutil.WriteFile(t, filepath.Join(dst, "Claude Extensions", "old", "x"), "old")
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	if _, err := os.Stat(filepath.Join(dst, "Claude Extensions", "e1", "manifest.json")); err != nil {
		t.Error("extension not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "Claude Extensions", "old")); err == nil {
		t.Error("stale extension kept in place")
	}
	if found, _ := filepath.Glob(filepath.Join(bk, "*", "Claude Extensions", "old", "x")); len(found) != 1 {
		t.Errorf("replaced extensions not backed up: %v", found)
	}
}

func TestSyncSettingsScratchUnion(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "scratch-workspaces", "a1", "o1", "s1", "a.txt"), "a")
	testutil.WriteFile(t, filepath.Join(src, "scratch-workspaces", "a1", "o1", "s1", "b.txt"), "src b")
	testutil.WriteFile(t, filepath.Join(dst, "scratch-workspaces", "a1", "o1", "s1", "b.txt"), "dst b")
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	if b, _ := os.ReadFile(filepath.Join(dst, "scratch-workspaces", "a1", "o1", "s1", "a.txt")); string(b) != "a" {
		t.Error("missing scratch file not copied")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "scratch-workspaces", "a1", "o1", "s1", "b.txt")); string(b) != "dst b" {
		t.Error("existing scratch file overwritten")
	}
}

func TestSyncSettingsNeverTouchesCredentials(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "Cookies"), "SECRET-COOKIE")
	testutil.WriteFile(t, filepath.Join(src, "Local Storage", "leveldb", "x"), "SECRET-LS")
	testutil.WriteFile(t, filepath.Join(src, "config.json"), `{"oauth:tokenCache":"SECRET-TOKEN"}`)
	testutil.WriteFile(t, filepath.Join(src, cfg), `{"preferences":{}}`)
	testutil.WriteFile(t, filepath.Join(dst, "Cookies"), "DST-COOKIE")
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	if b, _ := os.ReadFile(filepath.Join(dst, "Cookies")); string(b) != "DST-COOKIE" {
		t.Error("Cookies changed")
	}
	for _, p := range []string{"Local Storage", "config.json"} {
		if _, err := os.Stat(filepath.Join(dst, p)); err == nil {
			t.Errorf("%s copied", p)
		}
	}
	for _, root := range []string{dst, bk} {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.Mode().IsRegular() {
				if b, _ := os.ReadFile(path); strings.Contains(string(b), "SECRET") {
					t.Errorf("credential content found in %s", path)
				}
			}
			return nil
		})
	}
}
