package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/handoff"
	"github.com/zahT3/hotseat/internal/paths"
)

var handoffRunner handoff.Runner = handoff.OSA{}

// cmdHandoff runs detached from Claude Code (started by the StopFailure
// hook), so errors go to <state>/handoff.log instead of a terminal.
func cmdHandoff(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var r handoff.Request
	fs.StringVar(&r.From, "from", "", "")
	fs.StringVar(&r.To, "to", "", "")
	fs.StringVar(&r.SessionID, "session", "", "")
	fs.StringVar(&r.Cwd, "cwd", "", "")
	fs.StringVar(&r.Message, "message", "", "")
	fs.IntVar(&r.Tokens, "tokens", 0, "")
	fs.Int64Var(&r.ResetsAt, "resets", 0, "")
	fs.BoolVar(&r.Desktop, "desktop", false, "")
	err := fs.Parse(args)
	if err == nil {
		var reg account.Registry
		if reg, err = account.LoadRegistry(p); err == nil {
			err = handoff.Offer(p, reg, r, handoffRunner)
		}
	}
	if err != nil {
		logHandoff(p, fmt.Sprintf("%s → %s (session %s): %v", r.From, r.To, r.SessionID, err))
		return fail(stderr, err)
	}
	return 0
}

func logHandoff(p paths.Paths, line string) {
	if os.MkdirAll(p.StateDir, 0o700) != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(p.StateDir, "handoff.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), line)
}
