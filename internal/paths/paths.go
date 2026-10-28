// Package paths derives every file location Roster uses from $HOME and
// $ROSTER_HOME, so tests can redirect everything to temp dirs.
package paths

import (
	"errors"
	"os"
	"path/filepath"
)

type Paths struct {
	Home      string
	RosterDir string
}

func FromEnv() (Paths, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return Paths{}, errors.New("HOME is not set")
	}
	rosterDir := os.Getenv("ROSTER_HOME")
	if rosterDir == "" {
		rosterDir = filepath.Join(home, ".config", "roster")
	}
	return Paths{Home: filepath.Clean(home), RosterDir: filepath.Clean(rosterDir)}, nil
}

// CoreDir holds the single real copy of shared work data.
func (p Paths) CoreDir() string { return filepath.Join(p.Home, ".claude") }

// DefaultClaudeJSON is where Claude Code keeps .claude.json when
// CLAUDE_CONFIG_DIR is unset.
func (p Paths) DefaultClaudeJSON() string { return filepath.Join(p.Home, ".claude.json") }

func (p Paths) RegistryFile() string  { return filepath.Join(p.RosterDir, "accounts.json") }
func (p Paths) CurrentFile() string   { return filepath.Join(p.RosterDir, "current") }
func (p Paths) OverridesFile() string { return filepath.Join(p.RosterDir, "share-overrides.json") }
func (p Paths) BackupsDir() string    { return filepath.Join(p.RosterDir, "backups") }
func (p Paths) ShimDir() string       { return filepath.Join(p.RosterDir, "bin") }
func (p Paths) Zshrc() string         { return filepath.Join(p.Home, ".zshrc") }
