package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
)

func TestNoteSetShowAndClear(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	if code, _, stderr := run(t, "note", "claude2", "Work", "laptop"); code != 0 {
		t.Fatalf("note: %s", stderr)
	}
	reg, _ := account.LoadRegistry(p)
	if a, _ := reg.Find("claude2"); a.Note != "Work laptop" {
		t.Errorf("note = %q", a.Note)
	}
	_, v := panel(t)
	if v.Accounts[1].Note != "Work laptop" {
		t.Errorf("panel note = %q", v.Accounts[1].Note)
	}
	run(t, "init")
	reg, _ = account.LoadRegistry(p)
	if a, _ := reg.Find("claude2"); a.Note != "Work laptop" {
		t.Errorf("init dropped the note: %q", a.Note)
	}
	run(t, "note", "claude2")
	reg, _ = account.LoadRegistry(p)
	if a, _ := reg.Find("claude2"); a.Note != "" {
		t.Errorf("not cleared: %q", a.Note)
	}
	if code, _, _ := run(t, "note", "nope", "x"); code != 1 {
		t.Errorf("unknown account accepted")
	}
}

func TestPanelShowsEmail(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	f := filepath.Join(p.Home, ".claude2", ".claude.json")
	os.WriteFile(f, []byte(`{"oauthAccount":{"accountUuid":"b2","organizationUuid":"o2","emailAddress":"b@example.com"}}`), 0o600)
	_, v := panel(t)
	if v.Accounts[1].Email != "b@example.com" || v.Accounts[0].Email != "" {
		t.Errorf("emails %q %q", v.Accounts[0].Email, v.Accounts[1].Email)
	}
}
