package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/zahT3/hotseat/internal/paths"
)

// State records which account's data is at the real path and, for every
// profile, the account UUID last seen in it ("" until its first login).
type State struct {
	Active   string            `json:"active"`
	Profiles map[string]string `json:"profiles"`
}

func LoadState(p paths.Paths) (State, error) {
	b, err := os.ReadFile(p.DesktopStateFile())
	if errors.Is(err, os.ErrNotExist) {
		return State{Profiles: map[string]string{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, fmt.Errorf("parse %s: %w", p.DesktopStateFile(), err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]string{}
	}
	return s, nil
}

func SaveState(p paths.Paths, s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.DesktopStoreDir(), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(p.DesktopStoreDir(), ".state.tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p.DesktopStateFile())
}

// ProfileDir is where an account's Desktop data is: the real path when it
// is active, its parking spot otherwise.
func ProfileDir(p paths.Paths, s State, name string) string {
	if name == s.Active {
		return p.DesktopDataDir()
	}
	return p.DesktopProfileDir(name)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// Lock serialises everything that moves Desktop data (switch, recover,
// adopt), across processes: a limit dialog and `hotseat use` can race.
func Lock(p paths.Paths) (func(), error) {
	if err := os.MkdirAll(p.DesktopStoreDir(), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(p.DesktopStoreDir(), ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
