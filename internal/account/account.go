// Package account manages the registry of Claude accounts and reads their
// identity. It never touches credentials: from .claude.json it reads only
// oauthAccount.accountUuid and oauthAccount.organizationUuid.
package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zahT3/hotseat/internal/paths"
)

type Account struct {
	Name      string `json:"name"`
	ConfigDir string `json:"configDir"`
	// IsDefault marks the account living in $HOME/.claude. Claude Code must
	// run for it with CLAUDE_CONFIG_DIR unset, because the keychain entry is
	// keyed by config dir.
	IsDefault   bool   `json:"isDefault"`
	AccountUUID string `json:"accountUuid"`
	OrgUUID     string `json:"orgUuid"`
	// Note is the user's own label for the account.
	Note string `json:"note,omitempty"`
}

type Registry struct {
	Accounts []Account `json:"accounts"`
}

func (r Registry) Find(name string) (Account, bool) {
	for _, a := range r.Accounts {
		if a.Name == name {
			return a, true
		}
	}
	return Account{}, false
}

func (r Registry) Default() (Account, bool) {
	for _, a := range r.Accounts {
		if a.IsDefault {
			return a, true
		}
	}
	return Account{}, false
}

func LoadRegistry(p paths.Paths) (Registry, error) {
	b, err := os.ReadFile(p.RegistryFile())
	if errors.Is(err, os.ErrNotExist) {
		return Registry{}, nil
	}
	if err != nil {
		return Registry{}, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return Registry{}, fmt.Errorf("parse %s: %w", p.RegistryFile(), err)
	}
	return r, nil
}

func SaveRegistry(p paths.Paths, r Registry) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p.RegistryFile(), append(b, '\n'))
}

func ClaudeJSONPath(p paths.Paths, a Account) string {
	if a.IsDefault {
		return p.DefaultClaudeJSON()
	}
	return filepath.Join(a.ConfigDir, ".claude.json")
}

type Identity struct {
	AccountUUID string
	OrgUUID     string
}

var ErrNoIdentity = errors.New("no claude.ai login found")

func ReadIdentity(claudeJSON string) (Identity, error) {
	b, err := os.ReadFile(claudeJSON)
	if errors.Is(err, os.ErrNotExist) {
		return Identity{}, ErrNoIdentity
	}
	if err != nil {
		return Identity{}, err
	}
	var doc struct {
		OAuthAccount *struct {
			AccountUUID string `json:"accountUuid"`
			OrgUUID     string `json:"organizationUuid"`
		} `json:"oauthAccount"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return Identity{}, fmt.Errorf("parse %s: %w", claudeJSON, err)
	}
	if doc.OAuthAccount == nil || doc.OAuthAccount.AccountUUID == "" {
		return Identity{}, ErrNoIdentity
	}
	return Identity{AccountUUID: doc.OAuthAccount.AccountUUID, OrgUUID: doc.OAuthAccount.OrgUUID}, nil
}

// ReadEmail returns the login's email address from a .claude.json, or ""
// when there is none. Only the menu bar shows it, to tell accounts apart.
func ReadEmail(claudeJSON string) string {
	b, err := os.ReadFile(claudeJSON)
	if err != nil {
		return ""
	}
	var doc struct {
		OAuthAccount struct {
			Email string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return ""
	}
	return doc.OAuthAccount.Email
}

func CurrentName(p paths.Paths) (string, error) {
	b, err := os.ReadFile(p.CurrentFile())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func SetCurrent(p paths.Paths, name string) error {
	return writeFileAtomic(p.CurrentFile(), []byte(name+"\n"))
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
