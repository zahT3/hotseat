package cli

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/transcript"
	"github.com/zahT3/hotseat/internal/usage"
)

var clock = time.Now

// spawnHandoff starts `hotseat handoff` in its own session and returns at
// once, so the hook never holds up Claude Code while the dialog is open.
var spawnHandoff = func(args []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd := exec.Command(self, append([]string{"handoff"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, devnull, devnull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// cmdStopFailure handles the StopFailure hook (matcher rate_limit): record
// the limit, then offer a takeover. It writes nothing and always exits 0;
// Claude Code ignores StopFailure output anyway.
func cmdStopFailure(p paths.Paths, stdout, stderr io.Writer) int {
	raw, _ := io.ReadAll(io.LimitReader(stdin, 1<<20))
	var in struct {
		SessionID      string          `json:"session_id"`
		TranscriptPath string          `json:"transcript_path"`
		Cwd            string          `json:"cwd"`
		Error          json.RawMessage `json:"error"`
		Message        string          `json:"message"`
		LastAssistant  string          `json:"last_assistant_message"`
	}
	if json.Unmarshal(raw, &in) != nil || in.SessionID == "" {
		return 0
	}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return 0
	}
	from, ok := account.FromEnv(reg, os.Environ())
	if !ok {
		return 0
	}
	now := clock()

	// Claude Code passes the rendered notice as last_assistant_message;
	// older versions only leave it in the transcript. A rate_limit error
	// without a usage-limit notice is a transient 429 and is ignored.
	var msg string
	for _, m := range []string{in.LastAssistant, in.Message} {
		if transcript.IsLimitText(m) {
			msg = m
			break
		}
	}
	if msg == "" {
		m, ok := transcript.LastLimitMessage(in.TranscriptPath)
		if !ok {
			return 0
		}
		msg = m
	}
	var resets int64
	if t, ok := transcript.ParseResetTime(msg, now); ok {
		resets = t.Unix()
	}

	_ = usage.SetLimit(p, from.Name, usage.LimitHit{Message: msg, ResetsAt: resets, ObservedAt: now.Unix(), SessionID: in.SessionID, Cwd: in.Cwd})

	records, _ := loadRecords(p, reg)
	to, _ := usage.Recommend(reg, records, from.Name, now)
	_ = spawnHandoff([]string{
		"--from", from.Name, "--to", to, "--session", in.SessionID, "--cwd", in.Cwd,
		"--tokens", strconv.Itoa(transcript.ContextTokens(in.TranscriptPath)),
		"--resets", strconv.FormatInt(resets, 10), "--message", msg,
	})
	return 0
}
