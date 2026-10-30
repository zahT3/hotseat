package cli_test

import (
	"testing"

	"github.com/zahT3/hotseat/internal/account"
)

func TestIconSetShowAndClear(t *testing.T) {
	p := twoAccounts(t)
	run(t, "init")
	if code, _, stderr := run(t, "icon", "claude2", "briefcase.fill"); code != 0 {
		t.Fatalf("icon: %s", stderr)
	}
	_, v := panel(t)
	if v.Accounts[1].Icon != "briefcase.fill" {
		t.Errorf("panel icon = %q", v.Accounts[1].Icon)
	}
	run(t, "init")
	reg, _ := account.LoadRegistry(p)
	if a, _ := reg.Find("claude2"); a.Icon != "briefcase.fill" {
		t.Errorf("init dropped the icon: %q", a.Icon)
	}
	if code, _, _ := run(t, "icon", "claude2", "two words"); code != 1 {
		t.Error("accepted an icon with a space")
	}
	run(t, "icon", "claude2")
	reg, _ = account.LoadRegistry(p)
	if a, _ := reg.Find("claude2"); a.Icon != "" {
		t.Errorf("not cleared: %q", a.Icon)
	}
}
