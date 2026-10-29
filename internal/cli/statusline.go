package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/usage"
)

// stdin is what statusline and hook commands read Claude Code's JSON from.
var stdin io.Reader = os.Stdin

const chainTimeout = 800 * time.Millisecond

// cmdStatusline runs on every status bar render, so it must stay fast and
// must always print a line and exit 0, whatever goes wrong.
func cmdStatusline(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		return cmdStatuslineInstall(p, args, stdout, stderr)
	}
	raw, _ := io.ReadAll(io.LimitReader(stdin, 1<<20))
	var in struct {
		RateLimits usage.RateLimits `json:"rate_limits"`
	}
	_ = json.Unmarshal(raw, &in)

	mine := ""
	if reg, err := account.LoadRegistry(p); err == nil {
		if a, ok := account.FromEnv(reg, os.Environ()); ok {
			now := time.Now()
			_ = usage.ObserveRateLimits(p, a.Name, in.RateLimits, now)
			mine = ownStatus(p, a.Name, in.RateLimits, now)
		}
	}
	parts := []string{}
	if chained := runChain(p, raw); chained != "" {
		parts = append(parts, chained)
	}
	if mine != "" {
		parts = append(parts, mine)
	}
	fmt.Fprintln(stdout, strings.Join(parts, " | "))
	return 0
}

func ownStatus(p paths.Paths, name string, rl usage.RateLimits, now time.Time) string {
	five, seven := rl.FiveHour, rl.SevenDay
	if five == nil || seven == nil {
		if r, err := usage.Load(p, name); err == nil {
			if five == nil {
				five = effectiveRate(r.FiveHour, now)
			}
			if seven == nil {
				seven = effectiveRate(r.SevenDay, now)
			}
		}
	}
	s := name
	if five != nil {
		s += fmt.Sprintf(" · 5h %.0f%%", five.UsedPercentage)
	}
	if seven != nil {
		s += fmt.Sprintf(" · 7d %.0f%%", seven.UsedPercentage)
	}
	return s
}

func effectiveRate(w *usage.Window, now time.Time) *usage.RateWindow {
	pct, known, _ := usage.Effective(w, now)
	if !known {
		return nil
	}
	return &usage.RateWindow{UsedPercentage: pct, ResetsAt: w.ResetsAt}
}

// runChain runs the statusLine command the user had before Hotseat, feeding
// it the same JSON, and returns its first line.
func runChain(p paths.Paths, raw []byte) string {
	b, err := os.ReadFile(p.ChainFile())
	if err != nil {
		return ""
	}
	var chain struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(b, &chain) != nil || chain.Command == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), chainTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", chain.Command)
	cmd.Stdin = bytes.NewReader(raw)
	// Kill the whole process group on timeout and stop waiting for pipes a
	// leftover grandchild may still hold open.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 100 * time.Millisecond
	out, _ := cmd.Output()
	line, _, _ := bufio.NewReader(bytes.NewReader(out)).ReadLine()
	return strings.TrimSpace(string(line))
}
