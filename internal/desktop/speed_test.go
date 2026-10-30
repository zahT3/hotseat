package desktop_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/testutil"
)

// Extensions hold thousands of files; comparing them byte by byte on every
// switch took seconds. Same size and modification time counts as unchanged.
func TestSyncSettingsSkipsUnchangedExtensionsWithoutReading(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	sf := filepath.Join(src, "Claude Extensions", "e1", "index.js")
	df := filepath.Join(dst, "Claude Extensions", "e1", "index.js")
	testutil.WriteFile(t, sf, "same")
	testutil.WriteFile(t, df, "same")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	os.Chtimes(df, old, old)
	if err := desktop.SyncSettings(src, dst, "a1", "b2", bk); err != nil {
		t.Fatal(err)
	}
	si, _ := os.Stat(sf)
	di, _ := os.Stat(df)
	if !di.ModTime().Equal(si.ModTime()) {
		t.Errorf("identical file's mtime not aligned: %v vs %v", di.ModTime(), si.ModTime())
	}
	// Unreadable now: only a comparison by size and mtime can pass.
	os.Chmod(df, 0)
	defer os.Chmod(df, 0o600)
	if err := desktop.SyncSettings(src, dst, "a1", "b2", bk); err != nil {
		t.Fatal(err)
	}
	if found, _ := filepath.Glob(filepath.Join(bk, "*", "Claude Extensions")); len(found) != 0 {
		t.Errorf("unchanged extensions were replaced: %v", found)
	}
}

func TestSyncSettingsCopiedExtensionKeepsMtime(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	sf := filepath.Join(src, "Claude Extensions", "e1", "index.js")
	testutil.WriteFile(t, sf, "new")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	os.Chtimes(sf, old, old)
	desktop.SyncSettings(src, dst, "a1", "b2", bk)
	di, err := os.Stat(filepath.Join(dst, "Claude Extensions", "e1", "index.js"))
	if err != nil || !di.ModTime().Equal(old) {
		t.Errorf("copy mtime %v, want %v (err %v)", di, old, err)
	}
}

// Index files are ~120 KB, almost all of it a remote MCP config after the
// two fields Hotseat needs; reading stops once both are seen. The broken
// tail proves the rest is never parsed.
func TestSyncSessionsReadsOnlyTheFieldsItNeeds(t *testing.T) {
	src, dst, bk := t.TempDir(), t.TempDir(), t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "claude-code-sessions", "a1", "o1", "local_X.json"),
		`{"sessionId":"local_X","cliSessionId":"cX","lastActivityAt":200,"remoteMcpServersConfig":{"x":[1,2,`)
	tf := writeIndex(t, dst, "b2", "o2", "X", 100, "cX")
	rep, err := desktop.SyncSessions(src, dst, "b2", "o2", all("cX"), bk)
	if err != nil || rep.Updated != 1 {
		t.Errorf("err %v report %+v (last %d)", err, rep, readLast(t, tf))
	}
}

type aliveDriver struct {
	fakeDriver
	aliveFor int // Alive calls that still report the process
	gone     bool
}

func (a *aliveDriver) Alive(pid int) bool {
	a.aliveFor--
	a.gone = a.aliveFor < 0
	return !a.gone
}

func (a *aliveDriver) Procs() ([]desktop.Proc, error) {
	a.calls++
	if a.gone {
		return nil, nil
	}
	return a.procs, nil
}

// Listing processes costs about half a second (lsof); while waiting for
// Desktop to quit, a cheap per-PID check is polled instead.
func TestQuitAndWaitPollsPIDsCheaply(t *testing.T) {
	d := &aliveDriver{fakeDriver: fakeDriver{procs: claudeApp}, aliveFor: 3}
	start := time.Now()
	if err := desktop.QuitAndWait(d, home, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if d.calls != 2 || time.Since(start) > time.Second {
		t.Errorf("Procs called %d times, took %v", d.calls, time.Since(start))
	}
}
