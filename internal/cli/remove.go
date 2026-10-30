package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/shim"
)

// authLogout signs a config dir out through Claude Code itself, so its
// keychain entry does not outlive the account.
var authLogout = func(dir, shimDir string) error {
	self, _ := os.Executable()
	real, err := shim.FindRealClaude(os.Getenv("PATH"), self, shimDir)
	if err != nil {
		return err
	}
	cmd := exec.Command(real, "auth", "logout")
	cmd.Env = append(withoutEnv(os.Environ(), "CLAUDE_CONFIG_DIR"), "CLAUDE_CONFIG_DIR="+dir)
	return cmd.Run()
}

// cmdRemove takes an account out of Hotseat: it is signed out, and its
// config dir, Desktop profile and usage record move into a backup.
func cmdRemove(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	var pos []string
	for _, a := range positional(args) {
		if a != "--yes" {
			pos = append(pos, a)
		}
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "usage: hotseat remove <account> [--yes]")
		return 1
	}
	name := pos[0]
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	a, ok := reg.Find(name)
	if !ok {
		return fail(stderr, fmt.Errorf("unknown account %q", name))
	}
	if a.IsDefault {
		return fail(stderr, fmt.Errorf("%s is the default account: ~/.claude holds the shared work data of every account and cannot be removed", name))
	}
	if cur, _ := account.CurrentName(p); cur == name {
		return fail(stderr, fmt.Errorf("%s is the current account; switch to another one first", name))
	}
	s, err := desktop.LoadState(p)
	if err != nil {
		return fail(stderr, err)
	}
	if s.Active == name {
		return fail(stderr, fmt.Errorf("Claude Desktop runs as %s; switch Desktop to another account first", name))
	}
	_, hasProfile := s.Profiles[name]
	if !hasFlag(args, "--yes") {
		fmt.Fprintf(stdout, "This signs %s out and moves %s", name, tildePath(p, a.ConfigDir))
		if hasProfile {
			fmt.Fprint(stdout, " and its Claude Desktop profile")
		}
		fmt.Fprintln(stdout, " into a Hotseat backup. Close its terminals first. Re-run with --yes to do it.")
		return 1
	}

	backup := filepath.Join(p.BackupsDir(), "remove-"+name+"-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(backup, 0o700); err != nil {
		return fail(stderr, err)
	}
	loggedOut := authLogout(a.ConfigDir, p.ShimDir()) == nil
	if err := moveIfExists(a.ConfigDir, filepath.Join(backup, "config")); err != nil {
		return fail(stderr, err)
	}
	if hasProfile {
		if err := moveIfExists(p.DesktopProfileDir(name), filepath.Join(backup, "desktop")); err != nil {
			return fail(stderr, err)
		}
		delete(s.Profiles, name)
		if err := desktop.SaveState(p, s); err != nil {
			return fail(stderr, err)
		}
	}
	if err := moveIfExists(filepath.Join(p.UsageDir(), name+".json"), filepath.Join(backup, "usage.json")); err != nil {
		return fail(stderr, err)
	}
	var kept []account.Account
	for _, x := range reg.Accounts {
		if x.Name != name {
			kept = append(kept, x)
		}
	}
	reg.Accounts = kept
	if err := account.SaveRegistry(p, reg); err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, map[string]any{"removed": name, "backup": backup, "loggedOut": loggedOut})
	}
	fmt.Fprintf(stdout, "removed %s; its files are in %s\n", name, tildePath(p, backup))
	if !loggedOut {
		fmt.Fprintln(stdout, "could not sign it out with claude auth logout; its login stays in the keychain")
	}
	fmt.Fprintf(stdout, "if ~/.zshrc has an alias for %s, remove it by hand\n", name)
	return 0
}

func moveIfExists(from, to string) error {
	if _, err := os.Lstat(from); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return os.Rename(from, to)
}
