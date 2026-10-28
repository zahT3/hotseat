// Package share decides which entries of a Claude config dir are "work"
// (one real copy in ~/.claude, symlinked from every account dir) and which
// belong to the account, and moves account dirs to that layout.
package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/zahT3/roster/internal/paths"
)

type Class string

const (
	Shared  Class = "shared"
	Local   Class = "local"
	Pending Class = "pending"
	Unknown Class = "unknown"
)

var builtin = map[Class][]string{
	Shared: {"CLAUDE.md", "settings.json", "projects", "history.jsonl", "file-history", "tasks", "plans",
		"paste-cache", "skills", "commands", "agents", "output-styles", "rules", "workflows", "agent-memory",
		"keybindings.json", "themes"},
	Local: {".credentials.json", ".claude.json", ".claude.json.backup", "stats-cache.json", "remote-settings.json",
		"policy-limits.json", "backups", "shell-snapshots", "session-env", "statsig", "cache", "debug", "image-cache",
		"todos", "logs", "ide", "chrome", "mcp-needs-auth-cache.json", ".last-cleanup", ".last-update-result.json",
		".DS_Store", "usage-data", "feedback", "feedback-bundles", "settings.local.json"},
	// Sharing these needs an experiment first (absolute paths, per-process state).
	Pending: {"plugins", "sessions", "jobs", "daemon", "uploads"},
}

// neverShared are login-bound; sharing them would merge accounts.
var neverShared = []string{".credentials.json", ".claude.json"}

func SharedItems() []string {
	items := slices.Clone(builtin[Shared])
	slices.Sort(items)
	return items
}

type Classifier struct {
	overrides map[string]Class
}

func LoadClassifier(p paths.Paths) (Classifier, error) {
	o, err := loadOverrides(p)
	if err != nil {
		return Classifier{}, err
	}
	return Classifier{overrides: o}, nil
}

func (c Classifier) Classify(name string) Class {
	if cl, ok := c.overrides[name]; ok {
		return cl
	}
	for cl, names := range builtin {
		if slices.Contains(names, name) {
			return cl
		}
	}
	return Unknown
}

// SharedNames is the built-in shared list adjusted by user overrides, sorted.
func (c Classifier) SharedNames() []string {
	var names []string
	for _, n := range builtin[Shared] {
		if c.Classify(n) == Shared {
			names = append(names, n)
		}
	}
	for n, cl := range c.overrides {
		if cl == Shared && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return names
}

func SetOverride(p paths.Paths, name string, c Class) error {
	if c != Shared && c != Local {
		return fmt.Errorf("class must be %q or %q, got %q", Shared, Local, c)
	}
	if c == Shared && slices.Contains(neverShared, name) {
		return fmt.Errorf("%s holds the account login and cannot be shared", name)
	}
	if name == "" || name != filepath.Base(name) {
		return fmt.Errorf("invalid entry name %q", name)
	}
	o, err := loadOverrides(p)
	if err != nil {
		return err
	}
	o[name] = c
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.OverridesFile()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p.OverridesFile(), append(b, '\n'), 0o600)
}

func loadOverrides(p paths.Paths) (map[string]Class, error) {
	o := map[string]Class{}
	b, err := os.ReadFile(p.OverridesFile())
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p.OverridesFile(), err)
	}
	return o, nil
}
