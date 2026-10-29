package handoff_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/handoff"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/testutil"
)

type fakeRunner struct {
	press            string
	dialogText       string
	buttons          []string
	opened, notified []string
}

func (f *fakeRunner) Dialog(text string, buttons []string, def string) (string, error) {
	f.dialogText, f.buttons = text, buttons
	return f.press, nil
}
func (f *fakeRunner) Notify(title, text string) error {
	f.notified = append(f.notified, title+": "+text)
	return nil
}
func (f *fakeRunner) OpenTerminal(cmd string) error {
	f.opened = append(f.opened, cmd)
	return nil
}

func setup(t *testing.T) (paths.Paths, account.Registry) {
	t.Helper()
	p := testutil.FakeHome(t)
	reg := account.Registry{Accounts: []account.Account{
		{Name: "claude", ConfigDir: p.CoreDir(), IsDefault: true},
		{Name: "claude2", ConfigDir: filepath.Join(p.Home, ".claude2")},
	}}
	account.SaveRegistry(p, reg)
	account.SetCurrent(p, "claude2")
	return p, reg
}

var req = handoff.Request{From: "claude2", To: "claude", SessionID: "s1", Cwd: "/tmp/x", Tokens: 180002, ResetsAt: 2000000000,
	Message: "You've hit your weekly limit · resets Nov 1 at 6pm (Asia/Shanghai)"}

func TestOfferContinueSwitchesAndOpens(t *testing.T) {
	p, reg := setup(t)
	f := &fakeRunner{press: "Continue as claude"}
	if err := handoff.Offer(p, reg, req, f); err != nil {
		t.Fatal(err)
	}
	if cur, _ := account.CurrentName(p); cur != "claude" {
		t.Errorf("current = %q", cur)
	}
	if len(f.opened) != 1 || !strings.Contains(f.opened[0], "claude --resume s1") {
		t.Errorf("opened %v", f.opened)
	}
	for _, s := range []string{"claude2", "180k", "Do not keep typing"} {
		if !strings.Contains(f.dialogText, s) {
			t.Errorf("dialog text lacks %q:\n%s", s, f.dialogText)
		}
	}
}

func TestOfferDeclineChangesNothing(t *testing.T) {
	p, reg := setup(t)
	f := &fakeRunner{press: "Not now"}
	handoff.Offer(p, reg, req, f)
	if cur, _ := account.CurrentName(p); cur != "claude2" || len(f.opened) != 0 {
		t.Errorf("current %q opened %v", cur, f.opened)
	}
	f = &fakeRunner{press: ""} // gave up after the timeout
	handoff.Offer(p, reg, req, f)
	if cur, _ := account.CurrentName(p); cur != "claude2" || len(f.opened) != 0 {
		t.Errorf("timeout: current %q opened %v", cur, f.opened)
	}
}

func TestOfferNoAlternativeNotifies(t *testing.T) {
	p, reg := setup(t)
	r := req
	r.To = ""
	f := &fakeRunner{press: "Continue as "}
	handoff.Offer(p, reg, r, f)
	if f.dialogText != "" || len(f.notified) != 1 || len(f.opened) != 0 {
		t.Errorf("dialog %q notified %v opened %v", f.dialogText, f.notified, f.opened)
	}
}

func TestResumeCommandQuotesHostilePath(t *testing.T) {
	base := t.TempDir()
	cwd := filepath.Join(base, "a b", "it's $(touch PWNED) `touch PWNED2`")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(base, "bin")
	testutil.WriteFile(t, filepath.Join(bin, "claude"), "#!/bin/sh\necho \"args:$*\"\necho \"dir:$CLAUDE_CONFIG_DIR\"\n")
	os.Chmod(filepath.Join(bin, "claude"), 0o755)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	a := account.Account{Name: "w", ConfigDir: filepath.Join(base, "cfg dir")}
	cmd := handoff.ResumeCommand(a, cwd, "sess'1")
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s\ncmd: %s", err, out, cmd)
	}
	got := string(out)
	if !strings.Contains(got, "args:--resume sess'1") || !strings.Contains(got, "dir:"+a.ConfigDir) {
		t.Errorf("output %q\ncmd: %s", got, cmd)
	}
	for _, f := range []string{"PWNED", "PWNED2"} {
		if _, err := os.Stat(filepath.Join(cwd, f)); err == nil {
			t.Errorf("path content was executed (%s created)", f)
		}
	}
}

func TestResumeCommandDefaultUnsetsDir(t *testing.T) {
	cmd := handoff.ResumeCommand(account.Account{Name: "claude", IsDefault: true}, "/tmp", "s1")
	if !strings.Contains(cmd, "env -u CLAUDE_CONFIG_DIR claude --resume s1") {
		t.Errorf("cmd = %s", cmd)
	}
}

func TestAppleScriptEscaping(t *testing.T) {
	got := handoff.AppleScriptString("say \"hi\" \\ there\nline2")
	want := `"say \"hi\" \\ there\nline2"`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
