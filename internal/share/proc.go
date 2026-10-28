package share

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

var ErrClaudeRunning = errors.New("claude is running")

// RunningClaude lists pids of running Claude Code processes, which would race
// with Roster on history.jsonl and .claude.json.
func RunningClaude() ([]int, error) {
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, f := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(f); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}
