package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zahT3/hotseat/internal/paths"
)

// Journal records an in-flight swap so a crash between its two renames can
// be finished or undone.
type Journal struct {
	From string `json:"from"`
	To   string `json:"to"`
	Step string `json:"step"` // start, parked
}

// beforeSwapStep is a test hook; nil in production.
var beforeSwapStep func(step string) error

func hook(step string) error {
	if beforeSwapStep != nil {
		return beforeSwapStep(step)
	}
	return nil
}

// Swap parks the data at the real path as `from` and moves `to`'s parked
// data into the real path. Desktop must not be running.
func Swap(p paths.Paths, from, to string) error {
	active, parked, target := p.DesktopDataDir(), p.DesktopProfileDir(from), p.DesktopProfileDir(to)
	if exists(p.DesktopJournal()) {
		return errors.New("an earlier Desktop switch did not finish; run: hotseat desktop recover")
	}
	if !dirExists(active) {
		return fmt.Errorf("%s does not exist", active)
	}
	if exists(parked) {
		return fmt.Errorf("%s already exists; refusing to overwrite it", parked)
	}
	if !dirExists(target) {
		return fmt.Errorf("no Desktop data parked for %s at %s", to, target)
	}
	if err := os.MkdirAll(filepath.Dir(parked), 0o700); err != nil {
		return err
	}
	j := Journal{From: from, To: to, Step: "start"}
	if err := writeJournal(p, j); err != nil {
		return err
	}
	if err := hook("park"); err != nil {
		return err
	}
	if err := os.Rename(active, parked); err != nil {
		return err
	}
	j.Step = "parked"
	if err := writeJournal(p, j); err != nil {
		return err
	}
	if err := hook("activate"); err != nil {
		return err
	}
	if err := os.Rename(target, active); err != nil {
		return err
	}
	return os.Remove(p.DesktopJournal())
}

// Recover finishes or undoes a swap that was interrupted, and says what it
// did ("" when there was nothing to do).
func Recover(p paths.Paths) (string, error) {
	b, err := os.ReadFile(p.DesktopJournal())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var j Journal
	if err := json.Unmarshal(b, &j); err != nil {
		return "", fmt.Errorf("parse %s: %w", p.DesktopJournal(), err)
	}
	active, parked, target := p.DesktopDataDir(), p.DesktopProfileDir(j.From), p.DesktopProfileDir(j.To)
	switch {
	case dirExists(active) && !exists(parked):
		// The first rename never happened.
		return fmt.Sprintf("the switch to %s had not started; nothing changed", j.To), os.Remove(p.DesktopJournal())
	case !exists(active) && dirExists(target):
		if err := os.Rename(target, active); err != nil {
			return "", err
		}
		return fmt.Sprintf("finished switching Desktop from %s to %s", j.From, j.To), os.Remove(p.DesktopJournal())
	case dirExists(active) && exists(parked) && !exists(target):
		// Both renames happened; only the journal cleanup was missed.
		return fmt.Sprintf("the switch to %s had completed", j.To), os.Remove(p.DesktopJournal())
	default:
		return "", fmt.Errorf("cannot tell how far the switch from %s to %s got: real path present=%v, %s parked=%v, %s parked=%v; fix by hand, then delete %s",
			j.From, j.To, exists(active), j.From, exists(parked), j.To, exists(target), p.DesktopJournal())
	}
}

func writeJournal(p paths.Paths, j Journal) error {
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.DesktopStoreDir(), 0o700); err != nil {
		return err
	}
	tmp := p.DesktopJournal() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.DesktopJournal())
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
