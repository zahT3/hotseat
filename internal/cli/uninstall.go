package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/zahT3/hotseat/internal/paths"
)

// cmdUninstall takes out what Hotseat added to the user's shell and to
// Claude Code's settings. Data stays: shared links, backups, parked
// Desktop profiles and the app are listed, not touched.
func cmdUninstall(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	keeps := []string{
		"links between ~/.claude* dirs (undo with: hotseat share backups, then hotseat share rollback <id>)",
		tildePath(p, p.StateDir) + " (backups, parked Claude Desktop profiles, usage records)",
		"Hotseat.app (move it to the Trash yourself)",
	}
	if !hasFlag(args, "--yes") {
		fmt.Fprintln(stdout, "This removes the claude shim from ~/.zshrc, the Hotseat status line and the StopFailure hook.")
		fmt.Fprintln(stdout, "It keeps:")
		for _, k := range keeps {
			fmt.Fprintln(stdout, "  - "+k)
		}
		fmt.Fprintln(stdout, "Re-run with --yes to do it.")
		return 1
	}
	steps := []struct {
		name string
		run  func(paths.Paths) error
	}{
		{"shim", shimUninstall},
		{"statusline", statuslineUninstall},
		{"hook", hookUninstall},
	}
	results := map[string]string{}
	code := 0
	for _, s := range steps {
		switch err := s.run(p); {
		case err == nil:
			results[s.name] = "removed"
		case errors.As(err, new(leftAsIs)):
			results[s.name] = err.Error()
		default:
			results[s.name] = "failed: " + err.Error()
			code = 1
		}
	}
	if hasFlag(args, "--json") {
		if rc := writeJSON(stdout, stderr, map[string]any{"results": results, "kept": keeps}); rc != 0 {
			return rc
		}
		return code
	}
	for _, s := range steps {
		fmt.Fprintf(stdout, "%s: %s\n", s.name, results[s.name])
	}
	fmt.Fprintln(stdout, "kept:")
	for _, k := range keeps {
		fmt.Fprintln(stdout, "  - "+k)
	}
	return code
}
