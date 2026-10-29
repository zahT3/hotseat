package account

import (
	"path/filepath"
	"strings"
)

// FromEnv tells which registered account a Claude Code process (and the
// statusLine or hook it spawned) runs as. Claude Desktop sessions run with
// CLAUDE_CONFIG_DIR unset under whatever account Desktop is logged in to, so
// they are reported as unknown instead of as the default account.
func FromEnv(reg Registry, env []string) (Account, bool) {
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	dir := vars["CLAUDE_CONFIG_DIR"]
	if dir == "" {
		if vars["CLAUDE_CODE_ENTRYPOINT"] == "claude-desktop" {
			return Account{}, false
		}
		return reg.Default()
	}
	want := canonical(dir)
	for _, a := range reg.Accounts {
		if canonical(a.ConfigDir) == want {
			return a, true
		}
	}
	return Account{}, false
}

func canonical(dir string) string {
	dir = filepath.Clean(dir)
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		return r
	}
	return dir
}
