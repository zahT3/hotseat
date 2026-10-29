package share_test

import (
	"slices"
	"testing"

	"github.com/zahT3/hotseat/internal/share"
	"github.com/zahT3/hotseat/internal/testutil"
)

func TestClassifyBuiltins(t *testing.T) {
	p := testutil.FakeHome(t)
	c, err := share.LoadClassifier(p)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]share.Class{
		"projects":          share.Shared,
		"history.jsonl":     share.Shared,
		".credentials.json": share.Local,
		".claude.json":      share.Local,
		"plugins":           share.Pending,
		"scheduled-tasks":   share.Unknown,
	}
	for name, want := range cases {
		if got := c.Classify(name); got != want {
			t.Errorf("Classify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestSharedItemsSorted(t *testing.T) {
	items := share.SharedItems()
	if !slices.IsSorted(items) || !slices.Contains(items, "file-history") {
		t.Errorf("SharedItems = %v", items)
	}
}

func TestOverrideWins(t *testing.T) {
	p := testutil.FakeHome(t)
	if err := share.SetOverride(p, "scheduled-tasks", share.Shared); err != nil {
		t.Fatal(err)
	}
	if err := share.SetOverride(p, "themes", share.Local); err != nil {
		t.Fatal(err)
	}
	c, err := share.LoadClassifier(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Classify("scheduled-tasks"); got != share.Shared {
		t.Errorf("scheduled-tasks = %q", got)
	}
	names := c.SharedNames()
	if !slices.Contains(names, "scheduled-tasks") || slices.Contains(names, "themes") {
		t.Errorf("SharedNames = %v", names)
	}
}

func TestOverrideRejectsPending(t *testing.T) {
	p := testutil.FakeHome(t)
	if err := share.SetOverride(p, "plugins", share.Pending); err == nil {
		t.Fatal("expected error for pending")
	}
}

func TestOverrideRejectsDotNames(t *testing.T) {
	p := testutil.FakeHome(t)
	for _, name := range []string{".", "..", "-rf", "a/b"} {
		if err := share.SetOverride(p, name, share.Shared); err == nil {
			t.Errorf("SetOverride(%q) must be refused", name)
		}
	}
}

func TestHandEditedOverridesNeverShareCredentials(t *testing.T) {
	p := testutil.FakeHome(t)
	testutil.WriteFile(t, p.OverridesFile(), `{".credentials.json":"shared",".claude.json":"shared",".":"shared","..":"shared"}`)
	c, err := share.LoadClassifier(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range c.SharedNames() {
		if n == ".credentials.json" || n == ".claude.json" || n == "." || n == ".." {
			t.Errorf("SharedNames contains %q", n)
		}
	}
}

func TestOverrideNeverSharesCredentials(t *testing.T) {
	p := testutil.FakeHome(t)
	for _, name := range []string{".credentials.json", ".claude.json"} {
		if err := share.SetOverride(p, name, share.Shared); err == nil {
			t.Errorf("sharing %s must be refused", name)
		}
	}
}
