package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

type SwitchOptions struct {
	// Resume is "" (just open Desktop), "current" (the session focused last
	// on the outgoing account) or a Claude Code session UUID.
	Resume  string
	Timeout time.Duration
	// NoLaunch leaves Desktop closed after the swap (it was not running).
	NoLaunch bool
}

type SwitchResult struct {
	From     string     `json:"from"`
	To       string     `json:"to"`
	Resumed  string     `json:"resumed,omitempty"`
	Sessions SyncReport `json:"sessions"`
	Backup   string     `json:"backup,omitempty"`
	Noop     bool       `json:"noop,omitempty"`
}

// CurrentSession is the Code session focused most recently in an account's
// Desktop list, as a Claude Code session ID.
func CurrentSession(dataDir, acct, org string) (string, bool) {
	files, _ := filepath.Glob(filepath.Join(sessionsRoot(dataDir), acct, org, "local_*.json"))
	best, bestAt := "", int64(-1)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var v struct {
			CLISessionID  string `json:"cliSessionId"`
			LastFocusedAt int64  `json:"lastFocusedAt"`
		}
		if json.Unmarshal(b, &v) != nil || v.CLISessionID == "" {
			continue
		}
		if v.LastFocusedAt > bestAt {
			best, bestAt = v.CLISessionID, v.LastFocusedAt
		}
	}
	return best, best != ""
}

// Switch makes `to` the account Claude Desktop runs as: quit Desktop, carry
// the session list and settings over, swap the data dirs, reopen (with the
// given session when Resume is set). If anything fails after Desktop was
// quit, Desktop is reopened on the account it had.
func Switch(p paths.Paths, reg account.Registry, d Driver, to string, opts SwitchOptions) (res SwitchResult, err error) {
	unlock, err := Lock(p)
	if err != nil {
		return SwitchResult{}, err
	}
	defer unlock()
	if _, err := Recover(p); err != nil {
		return SwitchResult{}, err
	}
	s, err := LoadState(p)
	if err != nil {
		return SwitchResult{}, err
	}
	if s.Active == "" {
		return SwitchResult{}, errors.New("Claude Desktop is not managed yet; run: hotseat desktop adopt")
	}
	res = SwitchResult{From: s.Active, To: to}
	from, ok := reg.Find(s.Active)
	if !ok {
		return res, fmt.Errorf("active Desktop account %q is not registered", s.Active)
	}
	target, ok := reg.Find(to)
	if !ok {
		return res, fmt.Errorf("unknown account %q", to)
	}
	if target.OrgUUID == "" || from.OrgUUID == "" {
		return res, errors.New("an account has no organization ID in the registry; run hotseat init")
	}
	if to == s.Active && opts.Resume == "" {
		if err := checkIdentity(p, &s, from); err != nil {
			return res, err
		}
		res.Noop = true
		return res, nil
	}
	if to != s.Active {
		if !dirExists(p.DesktopProfileDir(to)) {
			return res, fmt.Errorf("no Desktop data for %s yet; run: hotseat desktop add %s", to, to)
		}
		if err := checkParked(p, s, target); err != nil {
			return res, err
		}
	}

	resume := opts.Resume
	if resume == "current" {
		resume, _ = CurrentSession(p.DesktopDataDir(), from.AccountUUID, from.OrgUUID)
	}
	wasRunning, err := quitAndWait(d, p.Home, opts.Timeout)
	if err != nil {
		return res, err
	}
	defer func() {
		if err != nil && wasRunning {
			_ = d.Launch()
		}
	}()
	if err := checkIdentity(p, &s, from); err != nil {
		return res, err
	}

	if to != s.Active {
		res.Backup = filepath.Join(p.BackupsDir(), "desktop-"+time.Now().Format("20060102-150405.000"))
		transcripts, err := TranscriptIDs(filepath.Join(p.CoreDir(), "projects"))
		if err != nil {
			return res, err
		}
		res.Sessions, err = SyncSessions(p.DesktopDataDir(), p.DesktopProfileDir(to), target.AccountUUID, target.OrgUUID, transcripts, res.Backup)
		if err != nil {
			return res, fmt.Errorf("sync sessions: %w", err)
		}
		if err := SyncSettings(p.DesktopDataDir(), p.DesktopProfileDir(to), from.AccountUUID, target.AccountUUID, res.Backup); err != nil {
			return res, fmt.Errorf("sync settings: %w", err)
		}
		if err := Swap(p, s.Active, to); err != nil {
			return res, err
		}
	}

	if opts.NoLaunch {
		return res, nil
	}
	if resume != "" {
		res.Resumed = resume
		return res, d.OpenURL("claude://resume?session=" + resume)
	}
	return res, d.Launch()
}

// checkParked refuses to bring in a parked profile that is logged in to a
// different account than the one it is filed under.
func checkParked(p paths.Paths, s State, target account.Account) error {
	actual, err := AccountUUID(p.DesktopProfileDir(target.Name))
	if err != nil {
		return err
	}
	want := s.Profiles[target.Name]
	if want == "" {
		want = target.AccountUUID
	}
	if actual != "" && actual != want {
		return fmt.Errorf("the Desktop data parked for %s is logged in to %q, not %q; not switching", target.Name, actual, want)
	}
	return nil
}

// checkIdentity refuses to park the data at the real path under the wrong
// name: it must still be logged in to the account Hotseat recorded (or, for
// a profile that was empty when added, to the registered account).
func checkIdentity(p paths.Paths, s *State, from account.Account) error {
	actual, err := AccountUUID(p.DesktopDataDir())
	if err != nil {
		return err
	}
	if actual == "" {
		return nil
	}
	recorded := s.Profiles[s.Active]
	if recorded != "" && actual != recorded {
		return fmt.Errorf("Claude Desktop is now logged in to %q, but Hotseat has it as %s (%s); log back in to %s or fix it with hotseat desktop status", actual, s.Active, recorded, s.Active)
	}
	if recorded == "" {
		if actual != from.AccountUUID {
			return fmt.Errorf("Claude Desktop was logged in to %q, not to %s (%s)", actual, from.Name, from.AccountUUID)
		}
		s.Profiles[s.Active] = actual
		return SaveState(p, *s)
	}
	return nil
}

// AddProfile prepares an empty Desktop data dir for an account that has
// never used Desktop here.
func AddProfile(p paths.Paths, name string) error {
	s, err := LoadState(p)
	if err != nil {
		return err
	}
	if s.Active == "" {
		return errors.New("Claude Desktop is not managed yet; run: hotseat desktop adopt")
	}
	if _, ok := s.Profiles[name]; ok || name == s.Active {
		return fmt.Errorf("%s already has Desktop data", name)
	}
	if exists(p.DesktopProfileDir(name)) {
		return fmt.Errorf("%s already exists; adopt it with hotseat desktop adopt %s <dir> or move it away first", p.DesktopProfileDir(name), name)
	}
	if err := os.MkdirAll(p.DesktopProfileDir(name), 0o700); err != nil {
		return err
	}
	s.Profiles[name] = ""
	return SaveState(p, s)
}
