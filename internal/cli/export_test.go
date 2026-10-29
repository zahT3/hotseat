package cli

import "io"

// SetStdin replaces the stdin commands read from and returns a restore func.
func SetStdin(r io.Reader) func() {
	old := stdin
	stdin = r
	return func() { stdin = old }
}
