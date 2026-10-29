package cli

import (
	"io"
	"time"
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
