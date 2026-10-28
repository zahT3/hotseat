package share

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/paths"
)

type ActionKind string

const (
	Link            ActionKind = "link"
	ReplaceWithLink ActionKind = "replace-with-link"
	MergeJSONL      ActionKind = "merge-jsonl"
	MergeDir        ActionKind = "merge-dir"
	Adopt           ActionKind = "adopt"
	Conflict        ActionKind = "conflict"
	SyncMCP         ActionKind = "sync-mcp"
)

type Action struct {
	Kind    ActionKind `json:"kind"`
	Account string     `json:"account"`
	Item    string     `json:"item"`
	Path    string     `json:"path"`
	Target  string     `json:"target"`
	Detail  string     `json:"detail,omitempty"`
}

type Plan struct {
	Actions []Action `json:"actions"`
	Skipped []string `json:"skipped,omitempty"`
}

func (pl Plan) Conflicts() int {
	n := 0
	for _, a := range pl.Actions {
		if a.Kind == Conflict {
			n++
		}
	}
	return n
}

func MakePlan(p paths.Paths, reg account.Registry, c Classifier) (Plan, error) {
	def, ok := reg.Default()
	if !ok {
		return Plan{}, errors.New("no default account (~/.claude) registered; run `roster init`")
	}
	pl := Plan{Actions: []Action{}}
	items := c.SharedNames()
	// adoptedFrom maps an item missing from core to the account copy an
	// earlier account will move there, so later accounts are planned against
	// that copy instead of adopting their own on top of it.
	adoptedFrom := map[string]string{}
	for _, a := range reg.Accounts {
		if a.IsDefault {
			continue
		}
		if info, err := os.Stat(a.ConfigDir); err != nil || !info.IsDir() {
			pl.Skipped = append(pl.Skipped, fmt.Sprintf("%s: config dir %s does not exist", a.Name, a.ConfigDir))
			continue
		}
		for _, item := range items {
			act, err := planItem(p, a, item, adoptedFrom[item])
			if err != nil {
				return Plan{}, err
			}
			if act == nil {
				continue
			}
			if act.Kind == Adopt {
				adoptedFrom[item] = act.Path
			}
			pl.Actions = append(pl.Actions, *act)
		}
	}

	mcp, err := planMCP(p, reg, def)
	if err != nil {
		return Plan{}, err
	}
	if mcp != nil {
		pl.Actions = append(pl.Actions, *mcp)
	}
	return pl, nil
}

// planItem decides what to do with one shared entry of one account. When
// pending is set, core does not exist yet but that path will be moved there
// by an earlier action, so it stands in for core.
func planItem(p paths.Paths, a account.Account, item, pending string) (*Action, error) {
	core := filepath.Join(p.CoreDir(), item)
	acct := filepath.Join(a.ConfigDir, item)
	act := &Action{Account: a.Name, Item: item, Path: acct, Target: core}

	coreSrc := core
	if pending != "" {
		coreSrc = pending
	}
	coreInfo, coreErr := os.Stat(coreSrc)
	if coreErr != nil && !errors.Is(coreErr, os.ErrNotExist) {
		return nil, coreErr
	}
	coreExists := coreErr == nil
	acctInfo, acctErr := os.Lstat(acct)
	if acctErr != nil && !errors.Is(acctErr, os.ErrNotExist) {
		return nil, acctErr
	}
	acctExists := acctErr == nil

	switch {
	case !acctExists && !coreExists:
		return nil, nil
	case !acctExists:
		act.Kind = Link
		return act, nil
	case acctInfo.Mode()&os.ModeSymlink != 0:
		if coreExists && sameTarget(acct, core) {
			return nil, nil
		}
		if target, _ := os.Readlink(acct); pending != "" && target == core {
			return nil, nil
		}
		act.Kind = Conflict
		act.Detail = "symlink does not point at the shared copy"
		return act, nil
	case !coreExists:
		act.Kind = Adopt
		return act, nil
	}

	switch {
	case coreInfo.IsDir() && acctInfo.IsDir():
		act.Kind = MergeDir
	case coreInfo.IsDir() != acctInfo.IsDir():
		act.Kind = Conflict
		act.Detail = "one side is a file, the other a directory"
	case item == "history.jsonl":
		act.Kind = MergeJSONL
	default:
		same, err := sameFile(coreSrc, acct)
		if err != nil {
			return nil, err
		}
		if same {
			act.Kind = ReplaceWithLink
		} else {
			act.Kind = Conflict
			act.Detail = "file differs from the shared copy"
		}
	}
	return act, nil
}

func planMCP(p paths.Paths, reg account.Registry, def account.Account) (*Action, error) {
	var files []string
	for _, a := range reg.Accounts {
		files = append(files, account.ClaudeJSONPath(p, a))
	}
	merged, conflicts, err := mergedMCP(files, account.ClaudeJSONPath(p, def))
	if err != nil {
		return nil, err
	}
	differs := false
	for _, f := range files {
		servers, ok, err := readServers(f)
		if err != nil {
			return nil, err
		}
		if ok && !sameServers(servers, merged) {
			differs = true
		}
	}
	if !differs {
		return nil, nil
	}
	names := make([]string, 0, len(merged))
	for n := range merged {
		names = append(names, n)
	}
	slices.Sort(names)
	detail := "servers: " + strings.Join(names, ", ")
	if len(conflicts) > 0 {
		detail += fmt.Sprintf("; defined differently, keeping %s's: %s", def.Name, strings.Join(conflicts, ", "))
	}
	return &Action{Kind: SyncMCP, Item: "mcpServers", Detail: detail}, nil
}

func sameTarget(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func sameFile(a, b string) (bool, error) {
	ba, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ba, bb), nil
}
