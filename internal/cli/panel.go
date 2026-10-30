package cli

import (
	"fmt"
	"io"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/usage"
)

type panelDesktop struct {
	Managed         bool   `json:"managed"`
	Active          string `json:"active,omitempty"`
	Running         bool   `json:"running"`
	RunningKnown    bool   `json:"runningKnown"`
	PendingRecovery bool   `json:"pendingRecovery"`
}

type panelAccount struct {
	usageRow
	Desktop        string `json:"desktop"` // active, parked or none
	DesktopProblem string `json:"desktopProblem,omitempty"`
	Email          string `json:"email,omitempty"`
	Note           string `json:"note,omitempty"`
}

type panelView struct {
	GeneratedAt int64          `json:"generatedAt"`
	Current     string         `json:"current"`
	Recommend   string         `json:"recommend,omitempty"`
	Desktop     panelDesktop   `json:"desktop"`
	Accounts    []panelAccount `json:"accounts"`
}

// cmdPanel prints everything the menu bar app shows, in one call. Problems
// with an account or with Desktop are data here, not a failing exit code.
func cmdPanel(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	if !hasFlag(args, "--json") {
		fmt.Fprintln(stdout, "use --json; this command feeds the menu bar app")
		return 0
	}
	now := clock()
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return fail(stderr, err)
	}
	rows, err := usageRows(p, now)
	if err != nil {
		return fail(stderr, err)
	}
	cur, _ := account.CurrentName(p)
	v := panelView{GeneratedAt: now.Unix(), Current: cur, Accounts: []panelAccount{}}
	records, _ := loadRecords(p, reg)
	v.Recommend, _ = usage.Recommend(reg, records, cur, now)

	profiles := map[string]profileStatus{}
	if s, err := desktop.LoadState(p); err == nil && s.Active != "" {
		v.Desktop.Managed, v.Desktop.Active = true, s.Active
		list, err := desktopStatus(p)
		if err != nil {
			return fail(stderr, err)
		}
		for _, ps := range list {
			profiles[ps.Account] = ps
		}
	}
	if procs, err := desktopDriver.Procs(); err == nil {
		v.Desktop.RunningKnown = true
		v.Desktop.Running = len(desktop.Running(procs, p.Home)) > 0
	}
	v.Desktop.PendingRecovery = fileExists(p.DesktopJournal())

	for _, r := range rows {
		a := panelAccount{usageRow: r, Desktop: "none"}
		if acct, ok := reg.Find(r.Account); ok {
			a.Note = acct.Note
			a.Email = account.ReadEmail(account.ClaudeJSONPath(p, acct))
		}
		if ps, ok := profiles[r.Account]; ok {
			a.Desktop, a.DesktopProblem = "parked", ps.Problem
			if ps.Active {
				a.Desktop = "active"
			}
		}
		v.Accounts = append(v.Accounts, a)
	}
	return writeJSON(stdout, stderr, v)
}
