// Package handoff asks the user whether to continue a session that hit its
// usage limit on another account, and if they agree, makes that account
// current and resumes the session in a new Terminal window. Nothing changes
// unless the user clicks the continue button.
package handoff

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/shellq"
)

type Request struct {
	From, To, SessionID, Cwd, Message string
	Tokens                            int
	ResetsAt                          int64
}

type Runner interface {
	// Dialog returns the pressed button, or "" if it timed out.
	Dialog(text string, buttons []string, defaultButton string) (string, error)
	Notify(title, text string) error
	OpenTerminal(command string) error
}

func Offer(p paths.Paths, reg account.Registry, r Request, run Runner) error {
	resets := "an unknown time"
	if r.ResetsAt > 0 {
		resets = time.Unix(r.ResetsAt, 0).Local().Format("Jan 2 15:04")
	}
	if r.To == "" {
		return run.Notify(r.From+" hit its usage limit", "Resets "+resets+". No other account has quota left.")
	}
	to, ok := reg.Find(r.To)
	if !ok {
		return fmt.Errorf("account %q is not registered", r.To)
	}
	cont := "Continue as " + r.To
	text := fmt.Sprintf("%s hit its usage limit (resets %s).\n\nContinue this session as %s?\n\nContext is about %dk tokens; the first message on %s reads it again without cache.\n\nDo not keep typing in the old window.",
		r.From, resets, r.To, (r.Tokens+500)/1000, r.To)
	pressed, err := run.Dialog(text, []string{"Not now", cont}, cont)
	if err != nil || pressed != cont {
		return err
	}
	if err := account.SetCurrent(p, r.To); err != nil {
		return err
	}
	return run.OpenTerminal(ResumeCommand(to, r.Cwd, r.SessionID))
}

// ResumeCommand is the shell line that continues the session as a. Every
// value is quoted, so nothing in a path runs.
func ResumeCommand(a account.Account, cwd, sessionID string) string {
	env := "env -u CLAUDE_CONFIG_DIR"
	if !a.IsDefault {
		env = "CLAUDE_CONFIG_DIR=" + shellq.Quote(a.ConfigDir)
	}
	return fmt.Sprintf("cd %s && %s claude --resume %s", shellq.Quote(cwd), env, shellq.Quote(sessionID))
}

// AppleScriptString returns s as an AppleScript string literal.
func AppleScriptString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}

// OSA is the real Runner, driving macOS through osascript.
type OSA struct{}

func (OSA) Dialog(text string, buttons []string, def string) (string, error) {
	qs := make([]string, len(buttons))
	for i, b := range buttons {
		qs[i] = AppleScriptString(b)
	}
	script := fmt.Sprintf("display dialog %s buttons {%s} default button %s with title \"Hotseat\" giving up after 600",
		AppleScriptString(text), strings.Join(qs, ", "), AppleScriptString(def))
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return "", err
	}
	// "button returned:Continue as claude, gave up:false"
	s := strings.TrimSpace(string(out))
	if strings.HasSuffix(s, "gave up:true") {
		return "", nil
	}
	s = strings.TrimPrefix(s, "button returned:")
	if i := strings.LastIndex(s, ", gave up:"); i >= 0 {
		s = s[:i]
	}
	return s, nil
}

func (OSA) Notify(title, text string) error {
	return exec.Command("osascript", "-e",
		fmt.Sprintf("display notification %s with title %s", AppleScriptString(text), AppleScriptString(title))).Run()
}

func (OSA) OpenTerminal(command string) error {
	return exec.Command("osascript",
		"-e", `tell application "Terminal"`,
		"-e", "activate",
		"-e", "do script "+AppleScriptString(command),
		"-e", "end tell").Run()
}
