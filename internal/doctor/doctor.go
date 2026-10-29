// Package doctor reports where the account dirs drift from the shared layout,
// plus setup problems that would make switching accounts silently fail.
package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
	"github.com/zahT3/hotseat/internal/share"
)

type Finding struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Account string `json:"account,omitempty"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

const oauthTokenVar = "CLAUDE_CODE_OAUTH_TOKEN"

func Check(p paths.Paths, reg account.Registry, env []string) ([]Finding, error) {
	var out []Finding
	add := func(level, code, acct, path, msg string) {
		out = append(out, Finding{Level: level, Code: code, Account: acct, Path: path, Message: msg})
	}

	missing := map[string]bool{}
	for _, a := range reg.Accounts {
		if info, err := os.Stat(a.ConfigDir); err != nil || !info.IsDir() {
			missing[a.Name] = true
			add("error", "account-dir-missing", a.Name, a.ConfigDir, "config dir does not exist")
			continue
		}
		id, err := account.ReadIdentity(account.ClaudeJSONPath(p, a))
		switch {
		case errors.Is(err, account.ErrNoIdentity):
			hint := fmt.Sprintf("CLAUDE_CONFIG_DIR=%s claude auth login", a.ConfigDir)
			if a.IsDefault {
				// The default account must log in with the variable unset,
				// or the login lands in a different keychain entry.
				hint = "env -u CLAUDE_CONFIG_DIR claude auth login"
			}
			add("warn", "identity-missing", a.Name, a.ConfigDir, "not logged in; run: "+hint)
		case err != nil:
			return nil, err
		case id.AccountUUID != a.AccountUUID:
			add("error", "identity-mismatch", a.Name, a.ConfigDir,
				"this dir is now logged in to a different Claude account than the one registered")
		}
	}

	c, err := share.LoadClassifier(p)
	if err != nil {
		return nil, err
	}
	pl, err := share.MakePlan(p, reg, c)
	if err != nil {
		return nil, err
	}
	for _, act := range pl.Actions {
		if missing[act.Account] {
			continue
		}
		switch act.Kind {
		case share.Link:
			add("warn", "shared-missing", act.Account, act.Path, act.Item+" is not linked to the shared copy yet; run hotseat share apply")
		case share.ReplaceWithLink, share.MergeJSONL, share.MergeDir, share.Adopt:
			add("error", "shared-diverged", act.Account, act.Path, act.Item+" is a separate copy, not the shared one; run hotseat share apply")
		case share.Conflict:
			add("error", "shared-conflict", act.Account, act.Path, act.Item+": "+act.Detail+"; resolve by hand")
		case share.SyncMCP:
			add("warn", "mcp-diverged", "", "", "MCP servers differ between accounts ("+act.Detail+"); hotseat share apply syncs them as a union: every account gets every server, so a server removed from only one account comes back")
		}
	}

	for _, a := range reg.Accounts {
		if missing[a.Name] {
			continue
		}
		entries, err := os.ReadDir(a.ConfigDir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			path := filepath.Join(a.ConfigDir, e.Name())
			switch c.Classify(e.Name()) {
			case share.Unknown:
				add("warn", "unknown-entry", a.Name, path,
					fmt.Sprintf("hotseat does not know whether %s is work or account data; classify it with: hotseat share set %s shared|local", e.Name(), e.Name()))
			case share.Pending:
				add("info", "pending-entry", a.Name, path, e.Name()+" stays per account until sharing it is tested")
			}
		}
	}

	if v, ok := lookup(env, oauthTokenVar); ok && v != "" {
		add("warn", "oauth-token-env", "", "environment",
			oauthTokenVar+" is set in the environment; it overrides every account's login, so switching accounts would silently do nothing")
	}
	settings := filepath.Join(p.CoreDir(), "settings.json")
	if hasSettingsEnv(settings, oauthTokenVar) {
		add("warn", "oauth-token-env", "", settings,
			oauthTokenVar+" is set in the env block of settings.json; it overrides every account's login")
	}

	if f := shimFinding(p, env); f != nil {
		out = append(out, *f)
	}
	return out, nil
}

func hasSettingsEnv(settings, key string) bool {
	b, err := os.ReadFile(settings)
	if err != nil {
		return false
	}
	var doc struct {
		Env map[string]json.RawMessage `json:"env"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return false
	}
	_, ok := doc.Env[key]
	return ok
}

func shimFinding(p paths.Paths, env []string) *Finding {
	shim := filepath.Join(p.ShimDir(), "claude")
	shimTarget, err := filepath.EvalSymlinks(shim)
	if err != nil {
		return &Finding{Level: "info", Code: "shim-inactive", Path: shim, Message: "claude shim not installed; run hotseat shim install"}
	}
	pathEnv, _ := lookup(env, "PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, "claude")
		info, err := os.Stat(candidate)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil && resolved == shimTarget {
			return nil
		}
		return &Finding{Level: "info", Code: "shim-inactive", Path: candidate,
			Message: "`claude` on PATH is not the hotseat shim, so it ignores the current account; open a new shell after hotseat shim install"}
	}
	return &Finding{Level: "info", Code: "shim-inactive", Message: "no claude on PATH"}
}

func lookup(env []string, key string) (string, bool) {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v, true
		}
	}
	return "", false
}
