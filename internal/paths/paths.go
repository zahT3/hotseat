// Package paths derives every file location Hotseat uses from $HOME and
// $HOTSEAT_HOME, so tests can redirect everything to temp dirs.
package paths

import (
	"errors"
	"os"
	"path/filepath"
)

type Paths struct {
	Home     string
	StateDir string
}

func FromEnv() (Paths, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return Paths{}, errors.New("HOME is not set")
	}
	stateDir := os.Getenv("HOTSEAT_HOME")
	if stateDir == "" {
		stateDir = filepath.Join(home, ".config", "hotseat")
	}
	return Paths{Home: filepath.Clean(home), StateDir: filepath.Clean(stateDir)}, nil
}

// CoreDir holds the single real copy of shared work data.
func (p Paths) CoreDir() string { return filepath.Join(p.Home, ".claude") }

// DefaultClaudeJSON is where Claude Code keeps .claude.json when
// CLAUDE_CONFIG_DIR is unset.
func (p Paths) DefaultClaudeJSON() string { return filepath.Join(p.Home, ".claude.json") }

func (p Paths) RegistryFile() string  { return filepath.Join(p.StateDir, "accounts.json") }
func (p Paths) CurrentFile() string   { return filepath.Join(p.StateDir, "current") }
func (p Paths) OverridesFile() string { return filepath.Join(p.StateDir, "share-overrides.json") }
func (p Paths) BackupsDir() string    { return filepath.Join(p.StateDir, "backups") }
func (p Paths) ShimDir() string       { return filepath.Join(p.StateDir, "bin") }
func (p Paths) Zshrc() string         { return filepath.Join(p.Home, ".zshrc") }
