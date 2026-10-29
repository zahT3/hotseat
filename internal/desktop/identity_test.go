package desktop_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/testutil"
)

func TestAccountUUIDReadsOnlyThatKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"oauth:tokenCache":"SECRET-TOKEN","nested":{"lastKnownAccountUuid":"no"},"lastKnownAccountUuid":"a1","after":[1,2]}`), 0o600)
	got, err := desktop.AccountUUID(dir)
	if err != nil || got != "a1" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestAccountUUIDMissing(t *testing.T) {
	dir := t.TempDir()
	if got, err := desktop.AccountUUID(dir); got != "" || err != nil {
		t.Errorf("no file: %q %v", got, err)
	}
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"locale":"en"}`), 0o600)
	if got, err := desktop.AccountUUID(dir); got != "" || err != nil {
		t.Errorf("no key: %q %v", got, err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	p := testutil.FakeHome(t)
	want := desktop.State{Active: "claude", Profiles: map[string]string{"claude": "a1", "claude3": ""}}
	if err := desktop.SaveState(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := desktop.LoadState(p)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, %v", got, err)
	}
	p2 := testutil.FakeHome(t)
	if s, err := desktop.LoadState(p2); err != nil || s.Active != "" {
		t.Errorf("missing state: %+v %v", s, err)
	}
}
