package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/zahT3/roster/internal/account"
	"github.com/zahT3/roster/internal/paths"
)

type ApplyOptions struct {
	Force   bool
	Running func() ([]int, error)
}

type Manifest struct {
	ID         string `json:"id"`
	RolledBack bool   `json:"rolledBack,omitempty"`
	Steps      []Step `json:"steps"`
}

// Step records every change one action made, so Rollback can undo it. A
// failed action still records whatever it changed before failing.
type Step struct {
	Action         Action   `json:"action"`
	BackupPath     string   `json:"backupPath,omitempty"`
	CoreBackupPath string   `json:"coreBackupPath,omitempty"`
	AddedToCore    []string `json:"addedToCore,omitempty"`
	Conflicts      []string `json:"conflicts,omitempty"`
	Created        string   `json:"created,omitempty"`
	Adopted        bool     `json:"adopted,omitempty"`
}

// beforeStep is a test hook; nil in production.
var beforeStep func(i int)

func Apply(p paths.Paths, reg account.Registry, pl Plan, opts ApplyOptions) (Manifest, error) {
	if !opts.Force {
		running := opts.Running
		if running == nil {
			running = RunningClaude
		}
		pids, err := running()
		if err != nil {
			return Manifest{}, fmt.Errorf("check for running claude: %w", err)
		}
		if len(pids) > 0 {
			return Manifest{}, fmt.Errorf("%w (pids %v)", ErrClaudeRunning, pids)
		}
	}
	id, dir, err := newBackupDir(p)
	if err != nil {
		return Manifest{}, err
	}
	m := Manifest{ID: id, Steps: []Step{}}
	if err := saveManifest(dir, m); err != nil {
		return m, err
	}
	for i, a := range pl.Actions {
		if a.Kind == Conflict {
			continue
		}
		if beforeStep != nil {
			beforeStep(i)
		}
		steps, err := applyAction(p, reg, dir, i, a)
		m.Steps = append(m.Steps, steps...)
		if serr := saveManifest(dir, m); serr != nil && err == nil {
			err = serr
		}
		if err != nil {
			return m, fmt.Errorf("%s %s: %w (undo with: roster share rollback %s)", a.Account, a.Item, err, id)
		}
	}
	return m, nil
}

func applyAction(p paths.Paths, reg account.Registry, dir string, i int, a Action) ([]Step, error) {
	switch a.Kind {
	case Link:
		if err := os.Symlink(a.Target, a.Path); err != nil {
			return nil, err
		}
		return []Step{{Action: a, Created: a.Path}}, nil
	case ReplaceWithLink:
		return moveAndLink(dir, Step{Action: a})
	case MergeJSONL:
		return mergeJSONLStep(dir, i, a)
	case MergeDir:
		added, conflicts, err := mergeTree(a.Path, a.Target)
		s := Step{Action: a, AddedToCore: added, Conflicts: conflicts}
		if err != nil {
			if len(added) > 0 {
				return []Step{s}, err
			}
			return nil, err
		}
		return moveAndLink(dir, s)
	case Adopt:
		if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(a.Path, a.Target); err != nil {
			return nil, err
		}
		s := Step{Action: a, Adopted: true}
		if err := os.Symlink(a.Target, a.Path); err != nil {
			return []Step{s}, err
		}
		s.Created = a.Path
		return []Step{s}, nil
	case SyncMCP:
		return syncMCP(p, reg, dir, i)
	default:
		return nil, fmt.Errorf("unknown action %q", a.Kind)
	}
}

// moveAndLink moves the account's copy into the backup dir and replaces it
// with a symlink to the shared copy.
func moveAndLink(dir string, s Step) ([]Step, error) {
	backup := filepath.Join(dir, s.Action.Account, s.Action.Item)
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		return stepsIfChanged(s), err
	}
	if err := os.Rename(s.Action.Path, backup); err != nil {
		return stepsIfChanged(s), err
	}
	s.BackupPath = backup
	if err := os.Symlink(s.Action.Target, s.Action.Path); err != nil {
		return []Step{s}, err
	}
	s.Created = s.Action.Path
	return []Step{s}, nil
}

func stepsIfChanged(s Step) []Step {
	if s.CoreBackupPath != "" || len(s.AddedToCore) > 0 || s.BackupPath != "" || s.Created != "" || s.Adopted {
		return []Step{s}
	}
	return nil
}

func mergeJSONLStep(dir string, i int, a Action) ([]Step, error) {
	coreBytes, err := os.ReadFile(a.Target)
	if err != nil {
		return nil, err
	}
	acctBytes, err := os.ReadFile(a.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(a.Target)
	if err != nil {
		return nil, err
	}
	coreBackup := filepath.Join(dir, "core", fmt.Sprintf("%d-%s", i, a.Item))
	if err := os.MkdirAll(filepath.Dir(coreBackup), 0o700); err != nil {
		return nil, err
	}
	if err := copyFile(a.Target, coreBackup, info.Mode().Perm()); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(a.Target, mergeJSONL(coreBytes, acctBytes), info.Mode().Perm()); err != nil {
		return nil, err
	}
	return moveAndLink(dir, Step{Action: a, CoreBackupPath: coreBackup})
}

func syncMCP(p paths.Paths, reg account.Registry, dir string, i int) ([]Step, error) {
	def, ok := reg.Default()
	if !ok {
		return nil, errors.New("no default account registered")
	}
	var files []string
	for _, a := range reg.Accounts {
		files = append(files, account.ClaudeJSONPath(p, a))
	}
	merged, _, err := mergedMCP(files, account.ClaudeJSONPath(p, def))
	if err != nil {
		return nil, err
	}
	var steps []Step
	for n, a := range reg.Accounts {
		f := files[n]
		servers, ok, err := readServers(f)
		if err != nil {
			return steps, err
		}
		if !ok || sameServers(servers, merged) {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			return steps, err
		}
		backup := filepath.Join(dir, "mcp", fmt.Sprintf("%d-%s.claude.json", i, a.Name))
		if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
			return steps, err
		}
		if err := copyFile(f, backup, info.Mode().Perm()); err != nil {
			return steps, err
		}
		if err := writeServers(f, merged, info.Mode().Perm()); err != nil {
			return steps, err
		}
		steps = append(steps, Step{
			Action:     Action{Kind: SyncMCP, Account: a.Name, Item: "mcpServers", Path: f},
			BackupPath: backup,
		})
	}
	return steps, nil
}

// writeServers replaces only the mcpServers key; every other key keeps its
// value.
func writeServers(file string, servers map[string]json.RawMessage, perm os.FileMode) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}
	raw, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	doc["mcpServers"] = raw
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(file, append(out, '\n'), perm)
}

func Rollback(p paths.Paths, id string) error {
	dir := filepath.Join(p.BackupsDir(), id)
	m, err := loadManifest(dir)
	if err != nil {
		return err
	}
	if m.RolledBack {
		return fmt.Errorf("backup %s was already rolled back", id)
	}
	var errs []error
	for i := len(m.Steps) - 1; i >= 0; i-- {
		if err := undoStep(dir, i, m.Steps[i]); err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", m.Steps[i].Action.Account, m.Steps[i].Action.Item, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	m.RolledBack = true
	return saveManifest(dir, m)
}

func undoStep(dir string, i int, s Step) error {
	if s.Action.Kind == SyncMCP {
		b, err := os.ReadFile(s.BackupPath)
		if err != nil {
			return err
		}
		info, err := os.Stat(s.BackupPath)
		if err != nil {
			return err
		}
		return writeFileAtomic(s.Action.Path, b, info.Mode().Perm())
	}
	if s.Created != "" {
		info, err := os.Lstat(s.Created)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return err
		case info.Mode()&os.ModeSymlink == 0:
			return fmt.Errorf("%s is no longer a symlink; refusing to remove it", s.Created)
		default:
			if err := os.Remove(s.Created); err != nil {
				return err
			}
		}
	}
	if s.Adopted {
		if err := os.Rename(s.Action.Target, s.Action.Path); err != nil {
			return err
		}
	}
	if s.BackupPath != "" {
		if err := os.Rename(s.BackupPath, s.Action.Path); err != nil {
			return err
		}
	}
	if s.CoreBackupPath != "" {
		if err := os.Rename(s.CoreBackupPath, s.Action.Target); err != nil {
			return err
		}
	}
	for j, added := range s.AddedToCore {
		dest := filepath.Join(dir, "rollback-removed", fmt.Sprintf("%d-%d", i, j), filepath.Base(added))
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}
		if err := os.Rename(added, dest); err != nil {
			return err
		}
	}
	return nil
}

func ListBackups(p paths.Paths) ([]string, error) {
	entries, err := os.ReadDir(p.BackupsDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(p.BackupsDir(), e.Name(), "manifest.json")); err == nil {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func LoadManifest(p paths.Paths, id string) (Manifest, error) {
	return loadManifest(filepath.Join(p.BackupsDir(), id))
}

func newBackupDir(p paths.Paths) (string, string, error) {
	base := time.Now().Format("20060102-150405")
	id := base
	for n := 2; ; n++ {
		dir := filepath.Join(p.BackupsDir(), id)
		err := os.MkdirAll(p.BackupsDir(), 0o700)
		if err != nil {
			return "", "", err
		}
		err = os.Mkdir(dir, 0o700)
		if err == nil {
			return id, dir, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", "", err
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

func saveManifest(dir string, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0o600)
}

func loadManifest(dir string) (Manifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return m, nil
}
