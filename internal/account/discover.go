package account

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/zahT3/roster/internal/paths"
)

var aliasRe = regexp.MustCompile(`(?m)^\s*alias\s+([A-Za-z0-9_.-]+)=['"]?CLAUDE_CONFIG_DIR=["']?([^"'\s\\]+)["']?\s+claude\b`)

// ParseAliases maps absolute config dirs to the shell alias that launches
// claude with them, e.g. alias claude2='CLAUDE_CONFIG_DIR="$HOME/.claude2" claude'.
func ParseAliases(zshrc string) map[string]string {
	home := os.Getenv("HOME")
	out := map[string]string{}
	for _, m := range aliasRe.FindAllStringSubmatch(zshrc, -1) {
		dir := m[2]
		for _, prefix := range []string{"${HOME}", "$HOME", "~"} {
			if strings.HasPrefix(dir, prefix) {
				dir = home + strings.TrimPrefix(dir, prefix)
				break
			}
		}
		if !filepath.IsAbs(dir) {
			continue
		}
		out[filepath.Clean(dir)] = m[1]
	}
	return out
}

// Discover finds config dirs that hold a claude.ai login: $HOME/.claude plus
// every $HOME/.claude* directory. Dirs without oauthAccount (API-key or
// provider-bridge setups) are skipped.
func Discover(p paths.Paths) ([]Account, error) {
	zshrc, err := os.ReadFile(p.Zshrc())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	aliases := ParseAliases(string(zshrc))

	entries, err := os.ReadDir(p.Home)
	if err != nil {
		return nil, err
	}
	var found []Account
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".claude") {
			continue
		}
		dir := filepath.Join(p.Home, e.Name())
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		a := Account{ConfigDir: dir, IsDefault: dir == p.CoreDir()}
		id, err := ReadIdentity(ClaudeJSONPath(p, a))
		if errors.Is(err, ErrNoIdentity) {
			continue
		}
		if err != nil {
			return nil, err
		}
		a.AccountUUID, a.OrgUUID = id.AccountUUID, id.OrgUUID
		switch {
		case a.IsDefault:
			a.Name = "claude"
		case aliases[dir] != "":
			a.Name = aliases[dir]
		default:
			a.Name = strings.TrimPrefix(e.Name(), ".")
		}
		found = append(found, a)
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].IsDefault != found[j].IsDefault {
			return found[i].IsDefault
		}
		return found[i].Name < found[j].Name
	})
	return found, nil
}
