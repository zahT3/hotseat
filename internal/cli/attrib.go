package cli

import (
	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/desktop"
	"github.com/zahT3/hotseat/internal/paths"
)

// sessionAccount tells which account a Claude Code process runs as. CLI
// sessions are identified by CLAUDE_CONFIG_DIR; a Claude Desktop session is
// the active Desktop account, but only while the data at the real path is
// still logged in to that account.
func sessionAccount(p paths.Paths, reg account.Registry, env []string) (a account.Account, isDesktop, ok bool) {
	if a, ok := account.FromEnv(reg, env); ok {
		return a, false, true
	}
	vars := envMap(env)
	if vars["CLAUDE_CODE_ENTRYPOINT"] != "claude-desktop" || vars["CLAUDE_CONFIG_DIR"] != "" {
		return account.Account{}, false, false
	}
	s, err := desktop.LoadState(p)
	if err != nil || s.Active == "" {
		return account.Account{}, false, false
	}
	a, found := reg.Find(s.Active)
	if !found {
		return account.Account{}, false, false
	}
	if actual, err := desktop.AccountUUID(p.DesktopDataDir()); err != nil || actual != a.AccountUUID {
		return account.Account{}, false, false
	}
	return a, true, true
}

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	return m
}
