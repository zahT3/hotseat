package cli

import (
	"io"
	"time"

	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/handoff"
)

// SetStdin replaces the stdin commands read from and returns a restore func.
func SetStdin(r io.Reader) func() {
	old := stdin
	stdin = r
	return func() { stdin = old }
}

// SetSpawnHandoff replaces how the StopFailure hook starts the handoff.
func SetSpawnHandoff(fn func(args []string) error) func() {
	old := spawnHandoff
	spawnHandoff = fn
	return func() { spawnHandoff = old }
}

// SetClock fixes the time commands see.
func SetClock(fn func() time.Time) func() {
	old := clock
	clock = fn
	return func() { clock = old }
}

// SetHandoffRunner replaces the dialog/Terminal driver.
func SetHandoffRunner(r handoff.Runner) func() {
	old := handoffRunner
	handoffRunner = r
	return func() { handoffRunner = old }
}

// SetSelfCommand fixes how settings.json refers to this binary.
func SetSelfCommand(cmd string) func() {
	old := selfCommand
	selfCommand = func() (string, error) { return cmd, nil }
	return func() { selfCommand = old }
}

// SetDesktopDriver replaces the real Claude Desktop driver.
func SetDesktopDriver(d desktop.Driver) func() {
	old := desktopDriver
	desktopDriver = d
	return func() { desktopDriver = old }
}

// SetAuthLogout replaces running `claude auth logout` for a config dir.
func SetAuthLogout(fn func(dir string) error) func() {
	old := authLogout
	authLogout = func(dir, _ string) error { return fn(dir) }
	return func() { authLogout = old }
}

// StablePath exposes stablePath for tests.
func StablePath(p string) string { return stablePath(p) }
