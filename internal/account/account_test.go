package account_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/testutil"
)

func TestReadIdentityReadsOnlyOauthUUIDs(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".claude.json")
	body := `{"oauthAccount":{"accountUuid":"a1","organizationUuid":"o1","emailAddress":"x@y"},"mcpServers":{}}`
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := account.ReadIdentity(f)
	if err != nil {
		t.Fatal(err)
	}
	if id != (account.Identity{AccountUUID: "a1", OrgUUID: "o1"}) {
		t.Errorf("got %+v", id)
	}
}

func TestReadIdentityMissingOauth(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(f, []byte(`{"numStartups":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := account.ReadIdentity(f); !errors.Is(err, account.ErrNoIdentity) {
		t.Errorf("err = %v, want ErrNoIdentity", err)
	}
}

func TestReadIdentityMissingFile(t *testing.T) {
	_, err := account.ReadIdentity(filepath.Join(t.TempDir(), "nope.json"))
	if !errors.Is(err, account.ErrNoIdentity) {
		t.Errorf("err = %v, want ErrNoIdentity", err)
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	p := testutil.FakeHome(t)
	want := account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true, AccountUUID: "a1", OrgUUID: "o1"},
		{Name: "claude2", ConfigDir: filepath.Join(p.Home, ".claude2"), AccountUUID: "b2", OrgUUID: "o2"},
	}}
	if err := account.SaveRegistry(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := account.LoadRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	info, err := os.Stat(p.RegistryFile())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 600", info.Mode().Perm())
	}
	if a, ok := got.Find("claude2"); !ok || a.AccountUUID != "b2" {
		t.Errorf("Find(claude2) = %+v, %v", a, ok)
	}
	if a, ok := got.Default(); !ok || a.Name != "claude" {
		t.Errorf("Default() = %+v, %v", a, ok)
	}
}

func TestLoadRegistryMissingIsEmpty(t *testing.T) {
	p := testutil.FakeHome(t)
	r, err := account.LoadRegistry(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Accounts) != 0 {
		t.Errorf("got %d accounts", len(r.Accounts))
	}
}

func TestClaudeJSONPathDefaultVsOther(t *testing.T) {
	p := testutil.FakeHome(t)
	def := account.Account{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true}
	other := account.Account{Name: "claude2", ConfigDir: filepath.Join(p.Home, ".claude2")}
	if got := account.ClaudeJSONPath(p, def); got != filepath.Join(p.Home, ".claude.json") {
		t.Errorf("default: %q", got)
	}
	if got := account.ClaudeJSONPath(p, other); got != filepath.Join(p.Home, ".claude2", ".claude.json") {
		t.Errorf("other: %q", got)
	}
}

func TestCurrentMissingIsEmpty(t *testing.T) {
	p := testutil.FakeHome(t)
	name, err := account.CurrentName(p)
	if err != nil || name != "" {
		t.Errorf("CurrentName = %q, %v", name, err)
	}
}

func TestCurrentRoundTrip(t *testing.T) {
	p := testutil.FakeHome(t)
	if err := account.SetCurrent(p, "claude2"); err != nil {
		t.Fatal(err)
	}
	name, err := account.CurrentName(p)
	if err != nil || name != "claude2" {
		t.Errorf("CurrentName = %q, %v", name, err)
	}
}
