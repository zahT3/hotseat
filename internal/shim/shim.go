// Package shim is what runs when hotseat is invoked as `claude`: it points
// CLAUDE_CONFIG_DIR at the current account and execs the real claude. It must
// never stop claude from starting; on any problem it starts claude unchanged.
package shim

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

const envKey = "CLAUDE_CONFIG_DIR"

var (
	execFn           = syscall.Exec
	stderr io.Writer = os.Stderr
	exit             = os.Exit
)

func ResolveEnv(env []string, reg account.Registry, current string) ([]string, string) {
	if v, ok := lookup(env, envKey); ok && v != "" {
		return env, ""
	}
	if current == "" {
		return env, ""
	}
	a, ok := reg.Find(current)
	if !ok {
		return env, fmt.Sprintf("hotseat: current account %q not found; starting claude unchanged", current)
	}
	out := without(env, envKey)
	// The default account must run with CLAUDE_CONFIG_DIR unset: the keychain
	// entry is keyed by config dir, so setting it to ~/.claude could read a
	// different entry and look logged out.
	if !a.IsDefault {
		out = append(out, envKey+"="+a.ConfigDir)
	}
	return out, ""
}

func FindRealClaude(pathEnv, self, shimDir string) (string, error) {
	selfResolved := resolve(self)
	shimResolved := resolve(shimDir)
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		if resolve(dir) == shimResolved {
			continue
		}
		candidate := filepath.Join(dir, "claude")
		info, err := os.Stat(candidate)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		if resolve(candidate) == selfResolved {
			continue
		}
		return candidate, nil
	}
	return "", errors.New("real claude not found in PATH")
}

func Run(args []string) {
	env := os.Environ()
	warning := ""
	if p, err := paths.FromEnv(); err != nil {
		warning = fmt.Sprintf("hotseat: %v; starting claude unchanged", err)
	} else if reg, err := account.LoadRegistry(p); err != nil {
		warning = fmt.Sprintf("hotseat: cannot read accounts (%v); starting claude unchanged", err)
	} else if cur, err := account.CurrentName(p); err != nil {
		warning = fmt.Sprintf("hotseat: cannot read current account (%v); starting claude unchanged", err)
	} else {
		env, warning = ResolveEnv(env, reg, cur)
	}
	if warning != "" {
		fmt.Fprintln(stderr, warning)
	}

	self, _ := os.Executable()
	shimDir := ""
	if p, err := paths.FromEnv(); err == nil {
		shimDir = p.ShimDir()
	}
	real, err := FindRealClaude(os.Getenv("PATH"), self, shimDir)
	if err != nil {
		fmt.Fprintln(stderr, "hotseat: real claude not found in PATH")
		exit(127)
		return
	}
	if err := execFn(real, args, env); err != nil {
		fmt.Fprintf(stderr, "hotseat: exec %s: %v\n", real, err)
		exit(126)
	}
}

func resolve(path string) string {
	if path == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return filepath.Clean(path)
}

func lookup(env []string, key string) (string, bool) {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v, true
		}
	}
	return "", false
}

func without(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}
