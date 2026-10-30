package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/settings"
)

type setupView struct {
	Version      string `json:"version"`
	Accounts     int    `json:"accounts"`
	LoginsFound  int    `json:"loginsFound"`
	ShareActions int    `json:"shareActions"`
	Shim         bool   `json:"shim"`
	Statusline   bool   `json:"statusline"`
	Hook         bool   `json:"hook"`
	Desktop      string `json:"desktop"` // adopted, available or absent
}

// cmdSetup reports which install steps are done, for the app's setup
// window. Everything is read-only.
func cmdSetup(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if pos := positional(args); len(pos) != 1 || pos[0] != "status" {
		fmt.Fprintln(stderr, "usage: hotseat setup status [--json]")
		return 1
	}
	v := setupView{Version: Version, Desktop: "absent"}
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	v.Accounts = len(reg.Accounts)
	if found, err := account.Discover(p); err == nil {
		v.LoginsFound = len(found)
	}
	if v.Accounts > 0 {
		if _, pl, err := loadPlan(p); err == nil {
			v.ShareActions = len(pl.Actions)
		}
	}
	if content, err := readZshrc(p); err == nil {
		_, _, inZshrc := findBlock(content)
		_, linkErr := os.Lstat(filepath.Join(p.ShimDir(), "claude"))
		v.Shim = inZshrc && linkErr == nil
	}
	file := settingsPath(p)
	if cmd, _, err := settings.StatusLineCommand(file); err == nil {
		v.Statusline = isOurs(cmd, "statusline")
	}
	if cmds, err := settings.HookCommands(file, "StopFailure"); err == nil {
		for _, c := range cmds {
			v.Hook = v.Hook || isOurs(c, "hook stop-failure")
		}
	}
	if s, err := desktop.LoadState(p); err == nil && s.Active != "" {
		v.Desktop = "adopted"
	} else if dirExists(p.DesktopDataDir()) {
		v.Desktop = "available"
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, v)
	}
	mark := func(done bool) string {
		if done {
			return "✓"
		}
		return "·"
	}
	lines := []string{
		fmt.Sprintf("%s accounts registered: %d (logins found: %d)", mark(v.Accounts > 0), v.Accounts, v.LoginsFound),
		fmt.Sprintf("%s work data shared (%d pending actions)", mark(v.Accounts > 0 && v.ShareActions == 0), v.ShareActions),
		fmt.Sprintf("%s claude follows the current account (shim)", mark(v.Shim)),
		fmt.Sprintf("%s usage tracking and limit handoff", mark(v.Statusline && v.Hook)),
		fmt.Sprintf("%s Claude Desktop: %s", mark(v.Desktop != "available"), v.Desktop),
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return 0
}
