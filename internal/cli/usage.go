package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/usage"
)

type windowView struct {
	UsedPercent float64 `json:"usedPercent"`
	ResetsAt    int64   `json:"resetsAt"`
	ObservedAt  int64   `json:"observedAt"`
	Source      string  `json:"source"`
	Reset       bool    `json:"reset"`
}

type usageRow struct {
	Account     string          `json:"account"`
	Current     bool            `json:"current"`
	FiveHour    *windowView     `json:"fiveHour"`
	SevenDay    *windowView     `json:"sevenDay"`
	Limit       *usage.LimitHit `json:"limit"`
	LimitActive bool            `json:"limitActive"`
}

func view(w *usage.Window, now time.Time) *windowView {
	pct, known, reset := usage.Effective(w, now)
	if !known {
		return nil
	}
	return &windowView{UsedPercent: pct, ResetsAt: w.ResetsAt, ObservedAt: w.ObservedAt, Source: w.Source, Reset: reset}
}

func usageRows(p paths.Paths, now time.Time) ([]usageRow, error) {
	reg, err := account.LoadRegistry(p)
	if err != nil {
		return nil, err
	}
	cur, _ := account.CurrentName(p)
	samples, err := usage.ReadDesktopSamples(filepath.Join(p.DesktopDataDir(), "plan-usage-history.json"))
	if err != nil {
		samples = nil
	}
	rows := []usageRow{}
	for _, a := range reg.Accounts {
		r, err := usage.Load(p, a.Name)
		if err != nil {
			return nil, err
		}
		if s, ok := samples[a.OrgUUID]; ok {
			r = usage.MergeDesktop(r, &s)
		}
		rows = append(rows, usageRow{
			Account: a.Name, Current: a.Name == cur,
			FiveHour: view(r.FiveHour, now), SevenDay: view(r.SevenDay, now),
			Limit: r.Limit, LimitActive: r.Limit.Active(now),
		})
	}
	return rows, nil
}

func cmdUsage(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	now := clock()
	rows, err := usageRows(p, now)
	if err != nil {
		return fail(stderr, err)
	}
	if hasFlag(args, "--json") {
		return writeJSON(stdout, stderr, rows)
	}
	tw := tabwriter.NewWriter(stdout, 0, 2, 2, ' ', 0)
	for _, r := range rows {
		mark := " "
		if r.Current {
			mark = "*"
		}
		cols := []string{mark + " " + r.Account, fmtWindow("5h", r.FiveHour, now), fmtWindow("7d", r.SevenDay, now), fmtAge(r, now)}
		if r.LimitActive {
			until := "unknown"
			if r.Limit.ResetsAt > 0 {
				until = fmtWhen(r.Limit.ResetsAt, now)
			}
			cols = append(cols, "LIMIT until "+until)
		}
		fmt.Fprintln(tw, strings.Join(cols, "\t"))
	}
	tw.Flush()
	return 0
}

func fmtWindow(label string, w *windowView, now time.Time) string {
	if w == nil {
		return label + " ?"
	}
	if w.Reset {
		return label + " 0% (reset)"
	}
	s := fmt.Sprintf("%s %.0f%%", label, w.UsedPercent)
	if w.ResetsAt > 0 {
		s += " (resets " + fmtWhen(w.ResetsAt, now) + ")"
	}
	if w.Source == "desktop" {
		s += " (desktop)"
	}
	return s
}

func fmtWhen(unix int64, now time.Time) string {
	t := time.Unix(unix, 0).In(now.Location())
	if t.Sub(now) < 24*time.Hour {
		return t.Format("15:04")
	}
	return t.Format("Mon 15:04")
}

func fmtAge(r usageRow, now time.Time) string {
	var newest int64
	for _, w := range []*windowView{r.FiveHour, r.SevenDay} {
		if w != nil && w.ObservedAt > newest {
			newest = w.ObservedAt
		}
	}
	if newest == 0 {
		return "no data yet"
	}
	d := now.Sub(time.Unix(newest, 0))
	switch {
	case d < time.Minute:
		return "updated just now"
	case d < time.Hour:
		return fmt.Sprintf("updated %dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("updated %dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("updated %dd ago", int(d.Hours()/24))
	}
}
