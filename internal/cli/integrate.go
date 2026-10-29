package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/settings"
	"github.com/zahT3/hotseat/internal/shellq"
)

// selfCommand is how settings.json should invoke this binary.
func selfCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return shellq.Quote(exe), nil
}

// isOurs reports whether a settings.json command runs some hotseat binary
// with the given subcommand, even one installed at another path.
func isOurs(command, sub string) bool {
	self, _ := selfCommand()
	first, rest, _ := strings.Cut(strings.TrimSpace(command), " ")
	if strings.TrimSpace(rest) != sub {
		return false
	}
	return first == self || filepath.Base(strings.Trim(first, `'"`)) == "hotseat"
}

func settingsPath(p paths.Paths) string { return filepath.Join(p.CoreDir(), "settings.json") }

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

	var c chain
	b, err := os.ReadFile(p.ChainFile())
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &c); err != nil {
			return fail(stderr, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return fail(stderr, err)
	}
	if raw != nil && !isOurs(current, "statusline") {
		return fail(stderr, errors.New("statusLine is not Hotseat's (changed since install); left as is"))
	}
	var previous json.RawMessage
	if c.Previous != "" {
		previous = json.RawMessage(c.Previous)
	}
	if err := settings.RestoreStatusLine(file, previous, settingsBackup(p)); err != nil {
		return fail(stderr, err)
	}
	if err := os.Remove(p.ChainFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fail(stderr, err)
	}
	return report(stdout, stderr, jsonOut, map[string]bool{"removed": true}, "removed the Hotseat status line")
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
		self, err := selfCommand()
		if err != nil {
			return fail(stderr, err)
		}
		if err := settings.AddHook(file, "StopFailure", "rate_limit", self+" hook stop-failure", settingsBackup(p)); err != nil {
			return fail(stderr, err)
		}
		return report(stdout, stderr, jsonOut, map[string]string{"hook": self + " hook stop-failure"},
			"installed the StopFailure hook (rate_limit)")
	case "uninstall":
		cmds, err := settings.HookCommands(file, "StopFailure")
		if err != nil {
			return fail(stderr, err)
		}
		for _, c := range cmds {
			if isOurs(c, "hook stop-failure") {
				if err := settings.RemoveHook(file, "StopFailure", c, settingsBackup(p)); err != nil {
					return fail(stderr, err)
				}
			}
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

// cmdStopFailure is filled in with the limit handoff.
func cmdStopFailure(p paths.Paths, stdout, stderr io.Writer) int { return 0 }
