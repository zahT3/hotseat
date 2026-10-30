package desktop

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const bundleID = "com.anthropic.claudefordesktop"

type Proc struct {
	PID   int
	Exe   string
	Paths []string
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
		inData := strings.HasPrefix(p.Exe, dataDir)
		// Claude Code rewrites its process title, so its real executable
		// and working directory come from lsof.
		for _, path := range p.Paths {
			if strings.HasPrefix(path, dataDir) {
				inData = true
			}
		}
		if inBundle || inData {
			out = append(out, p)
		}
	}
	return out
}

// QuitAndWait asks Desktop to quit and waits for every process of it to
// exit. It never force-kills: a running Code session may be mid-task.
func QuitAndWait(d Driver, home string, timeout time.Duration) error {
	_, err := quitAndWait(d, home, timeout)
	return err
}

// quitAndWait is QuitAndWait that also reports whether Desktop was running.
func quitAndWait(d Driver, home string, timeout time.Duration) (bool, error) {
	procs, err := d.Procs()
	if err != nil {
		return false, err
	}
	running := Running(procs, home)
	if len(running) == 0 {
		return false, nil
	}
	return true, waitQuit(d, home, running, timeout)
}

// pidChecker is implemented by drivers that can tell cheaply whether a
// process still exists; listing every process costs about half a second.
type pidChecker interface {
	Alive(pid int) bool
}

func waitQuit(d Driver, home string, running []Proc, timeout time.Duration) error {
	if err := d.Quit(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	pc, cheap := d.(pidChecker)
	for {
		if cheap {
			for anyAlive(pc, running) && time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
			}
		}
		// A full listing still decides: helpers may have started meanwhile.
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
		running = left
		if !cheap {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func anyAlive(pc pidChecker, procs []Proc) bool {
	for _, p := range procs {
		if pc.Alive(p.PID) {
			return true
		}
	}
	return false
}

// appLister is implemented by drivers that can list processes without
// their open files: enough to tell whether Desktop is running, without the
// half second lsof costs.
type appLister interface {
	AppProcs() ([]Proc, error)
}

// AppProcs lists processes cheaply when the driver can, for status display;
// anything that moves Desktop's data uses the full Procs.
func AppProcs(d Driver) ([]Proc, error) {
	if l, ok := d.(appLister); ok {
		return l.AppProcs()
	}
	return d.Procs()
}

// MacDriver drives the real Claude Desktop.
type MacDriver struct{}

func (MacDriver) AppProcs() ([]Proc, error) { return psProcs() }

// Alive sends signal 0, which only checks that the process exists.
func (MacDriver) Alive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func (MacDriver) Procs() ([]Proc, error) {
	procs, err := psProcs()
	if err != nil {
		return nil, err
	}
	paths, err := openPaths()
	if err != nil {
		return nil, err
	}
	for i := range procs {
		procs[i].Paths = paths[procs[i].PID]
	}
	return procs, nil
}

func psProcs() ([]Proc, error) {
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

// openPaths maps pid to its executable and working directory as lsof sees
// them (about half a second for every process on a typical Mac).
func openPaths() (map[int][]string, error) {
	out, err := exec.Command("lsof", "-nP", "-w", "-d", "cwd,txt", "-Fpn").Output()
	var exitErr *exec.ExitError
	if err != nil && !(errors.As(err, &exitErr) && len(out) > 0) {
		return nil, err
	}
	m := map[int][]string{}
	pid := 0
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			m[pid] = append(m[pid], line[1:])
		}
	}
	return m, nil
}

func (MacDriver) Quit() error {
	return exec.Command("osascript", "-e", `tell application id "`+bundleID+`" to quit`).Run()
}

func (MacDriver) OpenURL(url string) error { return exec.Command("open", url).Run() }

func (MacDriver) Launch() error { return exec.Command("open", "-b", bundleID).Run() }
