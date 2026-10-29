package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/share"
	"github.com/zahT3/hotseat/internal/shim"
)

var accountNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func cmdAdd(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	jsonOut := hasFlag(args, "--json")
	name, dir, err := parseAddArgs(p, positional(args))
	if err != nil {
		return fail(stderr, err)
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	if _, taken := reg.Find(name); taken {
		return fail(stderr, fmt.Errorf("account %q already exists", name))
	}
	if dir == p.CoreDir() {
		return fail(stderr, fmt.Errorf("%s is already the default account's dir", dir))
	}
	for _, a := range reg.Accounts {
		if filepath.Clean(a.ConfigDir) == dir {
			return fail(stderr, fmt.Errorf("%s is already used by account %q", dir, a.Name))
		}
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fail(stderr, fmt.Errorf("%s is not empty; pick another --dir", dir))
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(stderr, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fail(stderr, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fail(stderr, err)
	}

	self, _ := os.Executable()
	real, err := shim.FindRealClaude(os.Getenv("PATH"), self, p.ShimDir())
	if err != nil {
		return fail(stderr, err)
	}
	// With --json, stdout carries only the result; login chatter goes to stderr.
	loginOut := stdout
	if jsonOut {
		loginOut = stderr
	}
	fmt.Fprintf(loginOut, "opening the official Claude login for %s ...\n", dir)
	login := exec.Command(real, "auth", "login")
	login.Env = append(withoutEnv(os.Environ(), "CLAUDE_CONFIG_DIR"), "CLAUDE_CONFIG_DIR="+dir)
	login.Stdin, login.Stdout, login.Stderr = os.Stdin, loginOut, stderr
	_ = login.Run() // a cancelled login shows up as a missing identity below

	a := account.Account{Name: name, ConfigDir: dir}
	id, err := account.ReadIdentity(account.ClaudeJSONPath(p, a))
	if errors.Is(err, account.ErrNoIdentity) {
		return fail(stderr, fmt.Errorf("login did not complete; %s was left in place, retry with: hotseat add %s --dir %s", dir, name, dir))
	}
	if err != nil {
		return fail(stderr, err)
	}
	for _, other := range reg.Accounts {
		if other.AccountUUID == id.AccountUUID {
			return fail(stderr, fmt.Errorf("this Claude account is already registered as %q; move %s to the Trash if you do not need it", other.Name, dir))
		}
	}
	a.AccountUUID, a.OrgUUID = id.AccountUUID, id.OrgUUID
	reg.Accounts = append(reg.Accounts, a)
	if err := account.SaveRegistry(p, reg); err != nil {
		return fail(stderr, err)
	}

	if err := linkNewAccount(p, reg, name); err != nil {
		return fail(stderr, err)
	}
	if jsonOut {
		return writeJSON(stdout, stderr, a)
	}
	fmt.Fprintf(stdout, "added %s (%s)\n", name, tildePath(p, dir))
	fmt.Fprintln(stdout, "MCP servers are copied to the new account the next time you run: hotseat share apply")
	return 0
}

// linkNewAccount links a fresh account dir to the shared work data. A fresh
// dir can only need Link actions; anything else means the dir is not fresh.
func linkNewAccount(p paths.Paths, reg account.Registry, name string) error {
	c, err := share.LoadClassifier(p)
	if err != nil {
		return err
	}
	pl, err := share.MakePlan(p, reg, c)
	if err != nil {
		return err
	}
	var mine share.Plan
	for _, act := range pl.Actions {
		if act.Account != name {
			continue
		}
		if act.Kind != share.Link {
			return fmt.Errorf("unexpected %s for %s in a fresh account dir; run hotseat doctor", act.Kind, act.Item)
		}
		mine.Actions = append(mine.Actions, act)
	}
	// Only symlinks inside the new dir are created, so running claude
	// sessions cannot race with it.
	_, err = share.Apply(p, reg, mine, share.ApplyOptions{Force: true})
	return err
}

func parseAddArgs(p paths.Paths, args []string) (name, dir string, err error) {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--dir" && i+1 < len(args):
			dir = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--"):
			return "", "", fmt.Errorf("unknown flag %s", args[i])
		case name == "":
			name = args[i]
		default:
			return "", "", errors.New("usage: hotseat add <name> [--dir <path>]")
		}
	}
	if !accountNameRe.MatchString(name) {
		return "", "", fmt.Errorf("invalid account name %q (letters, digits, . _ -)", name)
	}
	if dir == "" {
		dir = filepath.Join(p.Home, ".claude-"+name)
	}
	dir, err = filepath.Abs(dir)
	return name, dir, err
}

func withoutEnv(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}
