package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/account"
	"github.com/zahT3/hotseat/internal/paths"
)

type ApplyOptions struct {
	Force   bool
	Running func() ([]int, error)
}

type RollbackOptions struct {
	Force   bool
	Running func() ([]int, error)
}

type Manifest struct {
	ID         string `json:"id"`
	RolledBack bool   `json:"rolledBack,omitempty"`
	Steps      []Step `json:"steps"`
}

// Step records the changes one action made. Each field is written to the
// manifest before the change happens and cleared again if the change fails,
// so after a crash rollback finds every change that might have happened.
// Rollback clears each field as it undoes that change, so a failed rollback
// can be retried from where it stopped.
type Step struct {
	Action         Action   `json:"action"`
	BackupPath     string   `json:"backupPath,omitempty"`
	CoreBackupPath string   `json:"coreBackupPath,omitempty"`
	AddedToCore    []string `json:"addedToCore,omitempty"`
	Conflicts      []string `json:"conflicts,omitempty"`
	Created        string   `json:"created,omitempty"`
	Adopted        bool     `json:"adopted,omitempty"`
	Undone         bool     `json:"undone,omitempty"`
}

// beforeStep is a test hook; nil in production.
var beforeStep func(i int)

// journal keeps the manifest on disk in step with every change.
type journal struct {
	dir string
	m   Manifest
}

func (j *journal) begin(a Action) int {
	j.m.Steps = append(j.m.Steps, Step{Action: a})
	return len(j.m.Steps) - 1
}

func (j *journal) record(i int, change func(s *Step)) error {
	change(&j.m.Steps[i])
	return saveManifest(j.dir, j.m)
}

func checkRunning(force bool, running func() ([]int, error)) error {
	if force {
		return nil
	}
	if running == nil {
		running = RunningClaude
	}
	pids, err := running()
	if err != nil {
		return fmt.Errorf("check for running claude: %w", err)
	}
	if len(pids) > 0 {
		return fmt.Errorf("%w (pids %v)", ErrClaudeRunning, pids)
	}
	return nil
}

func Apply(p paths.Paths, reg account.Registry, pl Plan, opts ApplyOptions) (Manifest, error) {
	if err := checkRunning(opts.Force, opts.Running); err != nil {
		return Manifest{}, err
	}
	if len(pl.Actions) == pl.Conflicts() {
		return Manifest{Steps: []Step{}}, nil
	}
	id, dir, err := newBackupDir(p)
	if err != nil {
		return Manifest{}, err
	}
	j := &journal{dir: dir, m: Manifest{ID: id, Steps: []Step{}}}
	if err := saveManifest(dir, j.m); err != nil {
		return j.m, err
	}
	for i, a := range pl.Actions {
		if a.Kind == Conflict {
			continue
		}
		if beforeStep != nil {
			beforeStep(i)
		}
		if err := applyAction(p, reg, j, i, a); err != nil {
			return j.m, fmt.Errorf("%s %s: %w (undo with: hotseat share rollback %s)", a.Account, a.Item, err, id)
		}
	}
	return j.m, nil
}

func applyAction(p paths.Paths, reg account.Registry, j *journal, i int, a Action) error {
	switch a.Kind {
	case Link:
		return link(j, j.begin(a))
	case ReplaceWithLink:
		return moveAndLink(j, j.begin(a))
	case MergeJSONL:
		return mergeJSONLStep(j, j.begin(a), i)
	case MergeDir:
		idx := j.begin(a)
		conflicts, err := mergeTree(a.Path, a.Target, func(added string) error {
			return j.record(idx, func(s *Step) { s.AddedToCore = append(s.AddedToCore, added) })
		})
		if err != nil {
			return err
		}
		if err := j.record(idx, func(s *Step) { s.Conflicts = conflicts }); err != nil {
			return err
		}
		return moveAndLink(j, idx)
	case Adopt:
		idx := j.begin(a)
		if exists(a.Target) {
			return fmt.Errorf("%s appeared since the plan was made; refusing to overwrite it", a.Target)
		}
		if err := os.MkdirAll(filepath.Dir(a.Target), 0o755); err != nil {
			return err
		}
		if err := j.record(idx, func(s *Step) { s.Adopted = true }); err != nil {
			return err
		}
		if err := os.Rename(a.Path, a.Target); err != nil {
			j.record(idx, func(s *Step) { s.Adopted = false })
			return err
		}
		return link(j, idx)
	case SyncMCP:
		return syncMCP(p, reg, j, i)
	default:
		return fmt.Errorf("unknown action %q", a.Kind)
	}
}

func link(j *journal, idx int) error {
	a := j.m.Steps[idx].Action
	if err := j.record(idx, func(s *Step) { s.Created = a.Path }); err != nil {
		return err
	}
	if err := os.Symlink(a.Target, a.Path); err != nil {
		j.record(idx, func(s *Step) { s.Created = "" })
		return err
	}
	return nil
}

// moveAndLink moves the account's copy into the backup dir and replaces it
// with a symlink to the shared copy.
func moveAndLink(j *journal, idx int) error {
	a := j.m.Steps[idx].Action
	backup := filepath.Join(j.dir, a.Account, a.Item)
	if exists(backup) {
		return fmt.Errorf("backup path %s already exists", backup)
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		return err
	}
	if err := j.record(idx, func(s *Step) { s.BackupPath = backup }); err != nil {
		return err
	}
	if err := os.Rename(a.Path, backup); err != nil {
		j.record(idx, func(s *Step) { s.BackupPath = "" })
		return err
	}
	return link(j, idx)
}

func mergeJSONLStep(j *journal, idx, i int) error {
	a := j.m.Steps[idx].Action
	coreBytes, err := os.ReadFile(a.Target)
	if err != nil {
		return err
	}
	acctBytes, err := os.ReadFile(a.Path)
	if err != nil {
		return err
	}
	info, err := os.Stat(a.Target)
	if err != nil {
		return err
	}
	coreBackup := filepath.Join(j.dir, "core", fmt.Sprintf("%d-%s", i, a.Item))
	if err := os.MkdirAll(filepath.Dir(coreBackup), 0o700); err != nil {
		return err
	}
	if err := copyFile(a.Target, coreBackup, info.Mode().Perm()); err != nil {
		return err
	}
	if err := j.record(idx, func(s *Step) { s.CoreBackupPath = coreBackup }); err != nil {
		return err
	}
	if err := writeFileAtomic(throughLink(a.Target), mergeJSONL(coreBytes, acctBytes), info.Mode().Perm()); err != nil {
		j.record(idx, func(s *Step) { s.CoreBackupPath = "" })
		return err
	}
	return moveAndLink(j, idx)
}

func syncMCP(p paths.Paths, reg account.Registry, j *journal, i int) error {
	def, ok := reg.Default()
	if !ok {
		return errors.New("no default account registered")
	}
	var files []string
	for _, a := range reg.Accounts {
		files = append(files, account.ClaudeJSONPath(p, a))
	}
	merged, _, err := mergedMCP(files, account.ClaudeJSONPath(p, def))
	if err != nil {
		return err
	}
	for n, a := range reg.Accounts {
		f := files[n]
		servers, ok, err := readServers(f)
		if err != nil {
			return err
		}
		if !ok || sameServers(servers, merged) {
			continue
		}
		info, err := os.Stat(f)
		if err != nil {
			return err
		}
		backup := filepath.Join(j.dir, "mcp", fmt.Sprintf("%d-%s.claude.json", i, a.Name))
		if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
			return err
		}
		if err := copyFile(f, backup, info.Mode().Perm()); err != nil {
			return err
		}
		idx := j.begin(Action{Kind: SyncMCP, Account: a.Name, Item: "mcpServers", Path: f})
		if err := j.record(idx, func(s *Step) { s.BackupPath = backup }); err != nil {
			return err
		}
		if err := writeServers(f, merged, info.Mode().Perm()); err != nil {
			j.record(idx, func(s *Step) { s.BackupPath = "" })
			return err
		}
	}
	return nil
}

// Rollback undoes an apply in reverse order and stops at the first error, so
// a retry after fixing the cause continues where it stopped. Files that
// changed after the apply are moved aside into the backup rather than
// overwritten; their paths are returned.
func Rollback(p paths.Paths, id string, opts RollbackOptions) ([]string, error) {
	if err := checkRunning(opts.Force, opts.Running); err != nil {
		return nil, err
	}
	dir := filepath.Join(p.BackupsDir(), id)
	m, err := loadManifest(dir)
	if err != nil {
		return nil, err
	}
	if m.RolledBack {
		return nil, fmt.Errorf("backup %s was already rolled back", id)
	}
	if newer, err := newerApplied(p, id); err != nil {
		return nil, err
	} else if len(newer) > 0 {
		return nil, fmt.Errorf("roll back the newer backups first: %s", strings.Join(newer, ", "))
	}
	j := &journal{dir: dir, m: m}
	var moved []string
	for i := len(j.m.Steps) - 1; i >= 0; i-- {
		if j.m.Steps[i].Undone {
			continue
		}
		mv, err := undoStep(j, i)
		moved = append(moved, mv...)
		if err != nil {
			s := j.m.Steps[i]
			return moved, fmt.Errorf("%s %s: %w (fix the cause and run the rollback again)", s.Action.Account, s.Action.Item, err)
		}
	}
	j.m.RolledBack = true
	return moved, saveManifest(dir, j.m)
}

func undoStep(j *journal, i int) ([]string, error) {
	s := j.m.Steps[i]
	a := s.Action
	var moved []string

	if a.Kind == SyncMCP {
		if s.BackupPath != "" {
			if err := restoreServers(s.BackupPath, a.Path); err != nil {
				return nil, err
			}
		}
		return nil, j.record(i, func(s *Step) { s.BackupPath = ""; s.Undone = true })
	}

	if s.Created != "" {
		info, err := os.Lstat(s.Created)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return nil, err
		case info.Mode()&os.ModeSymlink == 0:
			return nil, fmt.Errorf("%s is no longer a symlink; refusing to remove it", s.Created)
		default:
			if err := os.Remove(s.Created); err != nil {
				return nil, err
			}
		}
		if err := j.record(i, func(s *Step) { s.Created = "" }); err != nil {
			return nil, err
		}
	}
	if s.Adopted {
		if exists(a.Target) {
			if exists(a.Path) {
				return nil, fmt.Errorf("%s exists; refusing to overwrite it with %s", a.Path, a.Target)
			}
			if err := os.Rename(a.Target, a.Path); err != nil {
				return nil, err
			}
		}
		if err := j.record(i, func(s *Step) { s.Adopted = false }); err != nil {
			return nil, err
		}
	}
	if s.BackupPath != "" {
		if exists(s.BackupPath) {
			if exists(a.Path) {
				return nil, fmt.Errorf("%s exists; refusing to overwrite it with the backup %s", a.Path, s.BackupPath)
			}
			if err := os.Rename(s.BackupPath, a.Path); err != nil {
				return nil, err
			}
		}
		if err := j.record(i, func(s *Step) { s.BackupPath = "" }); err != nil {
			return nil, err
		}
	}
	if s.CoreBackupPath != "" {
		if exists(s.CoreBackupPath) {
			target := throughLink(a.Target)
			if exists(target) {
				dest, err := moveAside(j.dir, fmt.Sprintf("%d-core", i), target)
				if err != nil {
					return moved, err
				}
				moved = append(moved, dest)
			}
			if err := os.Rename(s.CoreBackupPath, target); err != nil {
				return moved, err
			}
		}
		if err := j.record(i, func(s *Step) { s.CoreBackupPath = "" }); err != nil {
			return moved, err
		}
	}
	for len(j.m.Steps[i].AddedToCore) > 0 {
		added := j.m.Steps[i].AddedToCore[0]
		if exists(added) {
			if _, err := moveAside(j.dir, fmt.Sprintf("%d-added", i), added); err != nil {
				return moved, err
			}
		}
		if err := j.record(i, func(s *Step) { s.AddedToCore = s.AddedToCore[1:] }); err != nil {
			return moved, err
		}
	}
	return moved, j.record(i, func(s *Step) { s.Undone = true })
}

// moveAside moves path into <backup>/rollback-removed/<tag>/ under a name
// that does not exist yet, and returns where it went.
func moveAside(dir, tag, path string) (string, error) {
	base := filepath.Join(dir, "rollback-removed", tag)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	dest := filepath.Join(base, filepath.Base(path))
	for n := 2; exists(dest); n++ {
		dest = filepath.Join(base, fmt.Sprintf("%s.%d", filepath.Base(path), n))
	}
	return dest, os.Rename(path, dest)
}

// restoreServers puts back only the mcpServers key from a backed-up
// .claude.json, leaving every key Claude Code changed since then alone.
func restoreServers(backup, file string) error {
	servers, ok, err := readServers(backup)
	if err != nil || !ok {
		return err
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	return writeServers(file, servers, info.Mode().Perm())
}

// throughLink returns the file a symlink points at, so writes keep a
// symlinked shared file (e.g. a synced history.jsonl) a symlink.
func throughLink(path string) string {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if r, err := filepath.EvalSymlinks(path); err == nil {
			return r
		}
	}
	return path
}

// newerApplied lists backups newer than id that changed something and were
// not rolled back; undoing an older apply under them would mix states.
func newerApplied(p paths.Paths, id string) ([]string, error) {
	ids, err := ListBackups(p)
	if err != nil {
		return nil, err
	}
	var newer []string
	for _, other := range ids {
		if other <= id {
			continue
		}
		m, err := LoadManifest(p, other)
		if err != nil {
			return nil, err
		}
		if !m.RolledBack && len(m.Steps) > 0 {
			newer = append(newer, other)
		}
	}
	return newer, nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
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
	if err := os.MkdirAll(p.BackupsDir(), 0o700); err != nil {
		return "", "", err
	}
	base := time.Now().Format("20060102-150405")
	id := base
	for n := 2; ; n++ {
		dir := filepath.Join(p.BackupsDir(), id)
		err := os.Mkdir(dir, 0o700)
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
