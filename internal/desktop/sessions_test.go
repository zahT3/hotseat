package desktop_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/testutil"
)

func writeIndex(t *testing.T, dataDir, acct, org, id string, last int64, cli string) string {
	t.Helper()
	f := filepath.Join(dataDir, "claude-code-sessions", acct, org, "local_"+id+".json")
	b, _ := json.Marshal(map[string]any{"sessionId": "local_" + id, "cliSessionId": cli, "lastActivityAt": last, "title": "t-" + id})
	testutil.WriteFile(t, f, string(b))
	return f
}

func target(dst string) string { return filepath.Join(dst, "claude-code-sessions", "b2", "o2") }

func readLast(t *testing.T, f string) int64 {
	t.Helper()
	var v struct{ LastActivityAt int64 }
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatalf("read %s: %v", f, err)
	}
	json.Unmarshal(b, &v)
	return v.LastActivityAt
}

func all(ids ...string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestSyncSessionsUnionIntoTargetFolder(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	writeIndex(t, src, "a1", "o1", "A", 1, "cA")
	writeIndex(t, src, "c3", "o3", "D", 1, "cD")
	writeIndex(t, dst, "b2", "o2", "C", 1, "cC")
	rep, err := desktop.SyncSessions(src, dst, "b2", "o2", all("cA", "cC", "cD"), bk)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"A", "C", "D"} {
		if _, err := os.Stat(filepath.Join(target(dst), "local_"+id+".json")); err != nil {
			t.Errorf("%s missing in target", id)
		}
	}
	if rep.Copied != 2 {
		t.Errorf("report %+v", rep)
	}
}

func TestSyncSessionsNewerWins(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	writeIndex(t, src, "a1", "o1", "X", 200, "cX")
	tf := writeIndex(t, dst, "b2", "o2", "X", 100, "cX")
	rep, _ := desktop.SyncSessions(src, dst, "b2", "o2", all("cX"), bk)
	if readLast(t, tf) != 200 || rep.Updated != 1 {
		t.Errorf("not updated: last %d report %+v", readLast(t, tf), rep)
	}
	if matches, _ := filepath.Glob(filepath.Join(bk, "*", "local_X.json")); len(matches) != 1 {
		t.Errorf("older copy not backed up: %v", matches)
	}
	writeIndex(t, src, "a1", "o1", "X", 50, "cX")
	desktop.SyncSessions(src, dst, "b2", "o2", all("cX"), bk)
	if readLast(t, tf) != 200 {
		t.Errorf("older source overwrote newer target")
	}
}

func TestSyncSessionsTombstoneWins(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "claude-code-sessions", "a1", "o1", "deleted_Y"), "")
	writeIndex(t, src, "a1", "o1", "Z", 1, "cZ")
	writeIndex(t, dst, "b2", "o2", "Y", 1, "cY")
	rep, _ := desktop.SyncSessions(src, dst, "b2", "o2", all("cY", "cZ"), bk)
	if _, err := os.Stat(filepath.Join(target(dst), "local_Y.json")); err == nil {
		t.Error("tombstoned session still listed")
	}
	if _, err := os.Stat(filepath.Join(target(dst), "deleted_Y")); err != nil {
		t.Error("tombstone not propagated")
	}
	if rep.Tombstoned != 1 {
		t.Errorf("report %+v", rep)
	}
	if matches, _ := filepath.Glob(filepath.Join(bk, "*", "local_Y.json")); len(matches) != 1 {
		t.Errorf("tombstoned index not kept in backup: %v", matches)
	}
}

func TestSyncSessionsSkipsDeadTranscript(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	writeIndex(t, src, "a1", "o1", "G", 1, "gone")
	rep, _ := desktop.SyncSessions(src, dst, "b2", "o2", all(), bk)
	if _, err := os.Stat(filepath.Join(target(dst), "local_G.json")); err == nil || rep.SkippedDead != 1 {
		t.Errorf("dead session copied; report %+v", rep)
	}
}

func TestSyncSessionsIdempotent(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	for i := 0; i < 5; i++ {
		id := fmt.Sprint(i)
		writeIndex(t, src, "a1", "o1", id, int64(i), "c"+id)
	}
	ids := all("c0", "c1", "c2", "c3", "c4")
	desktop.SyncSessions(src, dst, "b2", "o2", ids, bk)
	rep, _ := desktop.SyncSessions(src, dst, "b2", "o2", ids, bk)
	if rep != (desktop.SyncReport{}) {
		t.Errorf("second run changed things: %+v", rep)
	}
}

func TestTranscriptIDs(t *testing.T) {
	proj := t.TempDir()
	testutil.WriteFile(t, filepath.Join(proj, "-Users-u-x", "abc.jsonl"), "{}")
	testutil.WriteFile(t, filepath.Join(proj, "-Users-u-y", "def.jsonl"), "{}")
	testutil.WriteFile(t, filepath.Join(proj, "-Users-u-y", "memory", "MEMORY.md"), "")
	got, err := desktop.TranscriptIDs(proj)
	if err != nil || !got["abc"] || !got["def"] || len(got) != 2 {
		t.Errorf("got %v %v", got, err)
	}
}
