package desktop

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const bundleID = "com.anthropic.claudefordesktop"

type Proc struct {
	PID int
	Exe string
}

// Driver is everything Hotseat does to the real Claude Desktop, so tests can
// replace it.
type Driver interface {
	Procs() ([]Proc, error)
	Quit() error
	OpenURL(url string) error
	Launch() error
}

var ErrStillRunning = errors.New("Claude Desktop is still running")

// Running filters the processes that may touch Desktop's data dir: anything
// from a Claude.app bundle (wherever an update put it) except the Chrome
// extension's native host, and anything run from the data dir itself.
func Running(procs []Proc, home string) []Proc {
	dataDir := home + "/Library/Application Support/Claude/"
	var out []Proc
	for _, p := range procs {
		inBundle := strings.Contains(p.Exe, "/Claude.app/Contents/") && !strings.HasSuffix(p.Exe, "/Helpers/chrome-native-host")
		if inBundle || strings.HasPrefix(p.Exe, dataDir) {
			out = append(out, p)
		}
	}
	return out
}

// QuitAndWait asks Desktop to quit and waits for every process of it to
// exit. It never force-kills: a running Code session may be mid-task.
func QuitAndWait(d Driver, home string, timeout time.Duration) error {
	procs, err := d.Procs()
	if err != nil {
		return err
	}
	if len(Running(procs, home)) == 0 {
		return nil
	}
	if err := d.Quit(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		procs, err := d.Procs()
		if err != nil {
			return err
		}
		left := Running(procs, home)
		if len(left) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			pids := make([]string, len(left))
			for i, p := range left {
				pids[i] = strconv.Itoa(p.PID)
			}
			return fmt.Errorf("%w (pids %s); quit it and retry", ErrStillRunning, strings.Join(pids, ", "))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// MacDriver drives the real Claude Desktop.
type MacDriver struct{}

func (MacDriver) Procs() ([]Proc, error) {
	out, err := exec.Command("ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var procs []Proc
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		pidStr, exe, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		procs = append(procs, Proc{PID: pid, Exe: strings.TrimSpace(exe)})
	}
	return procs, nil
}

func (MacDriver) Quit() error {
	return exec.Command("osascript", "-e", `tell application id "`+bundleID+`" to quit`).Run()
}

func (MacDriver) OpenURL(url string) error { return exec.Command("open", url).Run() }

func (MacDriver) Launch() error { return exec.Command("open", "-b", bundleID).Run() }
