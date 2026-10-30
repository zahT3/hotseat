package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/settings"
	"github.com/zahT3/hotseat/internal/shellq"
)

// selfCommand is how settings.json should invoke this binary.
var selfCommand = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return shellq.Quote(stablePath(exe)), nil
}

// isOurs reports whether a settings.json command runs some hotseat binary
// with the given subcommand, even one installed at another path or quoted
// because its path has spaces.
func isOurs(command, sub string) bool {
	word, rest, ok := firstWord(command)
	if !ok || rest != sub {
		return false
	}
	self, _ := selfCommand()
	selfWord, _, _ := firstWord(self)
	return word == selfWord || filepath.Base(word) == "hotseat"
}

// firstWord splits off the first shell word of s, undoing quotes and
// backslash escapes, and returns the trimmed rest.
func firstWord(s string) (word, rest string, ok bool) {
	s = strings.TrimSpace(s)
	var b strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				b.WriteByte(c)
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			} else if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else {
				b.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == ' ' || c == '\t':
			return b.String(), strings.TrimSpace(s[i:]), true
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), "", !inSingle && !inDouble
}

func settingsPath(p paths.Paths) string { return filepath.Join(p.CoreDir(), "settings.json") }

// ensureSettings creates an empty settings.json for a Claude Code that has
// never written one (common on a fresh Mac), so installing can edit it.
func ensureSettings(p paths.Paths) error {
	file := settingsPath(p)
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte("{}\n"), 0o644)
}

func settingsBackup(p paths.Paths) string {
	return filepath.Join(p.BackupsDir(), "settings-"+time.Now().Format("20060102-150405.000000")+".json")
}

// chain remembers the user's own statusLine. Previous keeps the original
// bytes as a string so uninstall can put them back exactly.
type chain struct {
	Command  string `json:"command"`
	Previous string `json:"previous"`
}

func cmdStatuslineInstall(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	jsonOut := hasFlag(args, "--json")
	args = positional(args)
	if len(args) != 1 || (args[0] != "install" && args[0] != "uninstall") {
		fmt.Fprintln(stderr, "usage: hotseat statusline [install|uninstall]")
		return 1
	}
	if args[0] == "install" {
		if err := ensureSettings(p); err != nil {
			return fail(stderr, err)
		}
	}
	file := settingsPath(p)
	current, raw, err := settings.StatusLineCommand(file)
	if err != nil {
		return fail(stderr, err)
	}
	if args[0] == "install" {
		self, err := selfCommand()
		if err != nil {
			return fail(stderr, err)
		}
		if raw != nil && !isOurs(current, "statusline") {
			b, _ := json.MarshalIndent(chain{Command: current, Previous: string(raw)}, "", "  ")
			if err := os.MkdirAll(p.StateDir, 0o700); err != nil {
				return fail(stderr, err)
			}
			if err := os.WriteFile(p.ChainFile(), append(b, '\n'), 0o600); err != nil {
				return fail(stderr, err)
			}
		}
		if _, err := settings.SetStatusLine(file, self+" statusline", settingsBackup(p)); err != nil {
			return fail(stderr, err)
		}
		return report(stdout, stderr, jsonOut, map[string]string{"statusLine": self + " statusline"},
			"installed the Hotseat status line; running sessions pick it up on their own")
	}

	if err := statuslineUninstall(p); err != nil {
		return fail(stderr, err)
	}
	return report(stdout, stderr, jsonOut, map[string]bool{"removed": true}, "removed the Hotseat status line")
}

// leftAsIs is an uninstall step that found the user's own change and kept it.
type leftAsIs string

func (e leftAsIs) Error() string { return string(e) }

// errNotOurs means the user changed the setting since Hotseat installed it.
var errNotOurs = leftAsIs("statusLine is not Hotseat's (changed since install); left as is")

// statuslineUninstall puts back the status line the user had before.
func statuslineUninstall(p paths.Paths) error {
	file := settingsPath(p)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		return nil // never installed: nothing to take out
	}
	current, raw, err := settings.StatusLineCommand(file)
	if err != nil {
		return err
	}
	var c chain
	b, err := os.ReadFile(p.ChainFile())
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &c); err != nil {
			return err
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	if raw != nil && !isOurs(current, "statusline") {
		return errNotOurs
	}
	if raw == nil && c.Previous != "" {
		// The user deleted the status line since install; putting their
		// old one back would undo that.
		if err := os.Remove(p.ChainFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return leftAsIs("statusLine was removed since install; left as is")
	}
	var previous json.RawMessage
	if c.Previous != "" {
		previous = json.RawMessage(c.Previous)
	}
	if err := settings.RestoreStatusLine(file, previous, settingsBackup(p)); err != nil {
		return err
	}
	if err := os.Remove(p.ChainFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// hookUninstall removes every StopFailure hook that runs hotseat.
func hookUninstall(p paths.Paths) error {
	file := settingsPath(p)
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	cmds, err := settings.HookCommands(file, "StopFailure")
	if err != nil {
		return err
	}
	for _, c := range cmds {
		if isOurs(c, "hook stop-failure") {
			if err := settings.RemoveHook(file, "StopFailure", c, settingsBackup(p)); err != nil {
				return err
			}
		}
	}
	return nil
}

func cmdHook(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	jsonOut := hasFlag(args, "--json")
	args = positional(args)
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: hotseat hook install|uninstall|stop-failure")
		return 1
	}
	file := settingsPath(p)
	switch args[0] {
	case "install":
		if err := ensureSettings(p); err != nil {
			return fail(stderr, err)
		}
		self, err := selfCommand()
		if err != nil {
			return fail(stderr, err)
		}
		cmd := self + " hook stop-failure"
		// A hotseat installed at another path would fire a second dialog.
		existing, err := settings.HookCommands(file, "StopFailure")
		if err != nil {
			return fail(stderr, err)
		}
		for _, c := range existing {
			if c != cmd && isOurs(c, "hook stop-failure") {
				if err := settings.RemoveHook(file, "StopFailure", c, settingsBackup(p)); err != nil {
					return fail(stderr, err)
				}
			}
		}
		if err := settings.AddHook(file, "StopFailure", "rate_limit", cmd, settingsBackup(p)); err != nil {
			return fail(stderr, err)
		}
		return report(stdout, stderr, jsonOut, map[string]string{"hook": self + " hook stop-failure"},
			"installed the StopFailure hook (rate_limit)")
	case "uninstall":
		if err := hookUninstall(p); err != nil {
			return fail(stderr, err)
		}
		return report(stdout, stderr, jsonOut, map[string]bool{"removed": true}, "removed the StopFailure hook")
	case "stop-failure":
		return cmdStopFailure(p, stdout, stderr)
	default:
		fmt.Fprintln(stderr, "usage: hotseat hook install|uninstall|stop-failure")
		return 1
	}
}

func report(stdout, stderr io.Writer, jsonOut bool, v any, text string) int {
	if jsonOut {
		return writeJSON(stdout, stderr, v)
	}
	fmt.Fprintln(stdout, text)
	return 0
}

var cellarRe = regexp.MustCompile(`^(.*)/Cellar/([^/]+)/[^/]+/(.*)$`)

// stablePath turns a Homebrew Cellar path, which changes with every upgrade
// and is deleted by cleanup, into the formula's stable opt link.
func stablePath(p string) string {
	return cellarRe.ReplaceAllString(p, "$1/opt/$2/$3")
}
