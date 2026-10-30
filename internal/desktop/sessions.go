package desktop

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SyncReport counts what SyncSessions changed in the target folder.
type SyncReport struct {
	Copied      int `json:"copied"`
	Updated     int `json:"updated"`
	Tombstoned  int `json:"tombstoned"`
	SkippedDead int `json:"skippedDead"`
}

type indexFile struct {
	path       string
	lastActive int64
	cliSession string
}

func sessionsRoot(dataDir string) string { return filepath.Join(dataDir, "claude-code-sessions") }

// SyncSessions makes the target account's Code session folder in dst
// (claude-code-sessions/<dstAcct>/<dstOrg>/, the only folder Desktop lists
// for that account) the union of every account folder in src and dst.
// The newest copy of a session wins, deletions (deleted_<id>) spread,
// sessions whose transcript is gone are not spread, and nothing is
// deleted: replaced files go to backup.
func SyncSessions(src, dst, dstAcct, dstOrg string, transcripts map[string]bool, backup string) (SyncReport, error) {
	var rep SyncReport
	target := filepath.Join(sessionsRoot(dst), dstAcct, dstOrg)
	locals := map[string][]indexFile{}
	tombs := map[string]string{} // id -> a path of its deleted_<id> marker
	for _, root := range []string{sessionsRoot(src), sessionsRoot(dst)} {
		folders, _ := filepath.Glob(filepath.Join(root, "*", "*"))
		for _, folder := range folders {
			entries, err := os.ReadDir(folder)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				path := filepath.Join(folder, name)
				switch {
				case strings.HasPrefix(name, "deleted_"):
					if _, ok := tombs[strings.TrimPrefix(name, "deleted_")]; !ok {
						tombs[strings.TrimPrefix(name, "deleted_")] = path
					}
				case strings.HasPrefix(name, "local_") && strings.HasSuffix(name, ".json"):
					id := strings.TrimSuffix(strings.TrimPrefix(name, "local_"), ".json")
					locals[id] = append(locals[id], readIndex(path))
				}
			}
		}
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return rep, err
	}

	for id, marker := range tombs {
		dstMarker := filepath.Join(target, "deleted_"+id)
		if !exists(dstMarker) {
			if err := copyTree(marker, dstMarker); err != nil {
				return rep, err
			}
		}
		listed := filepath.Join(target, "local_"+id+".json")
		if exists(listed) {
			if err := moveToBackup(backup, "sessions", listed); err != nil {
				return rep, err
			}
			rep.Tombstoned++
		}
	}

	for id, copies := range locals {
		if _, dead := tombs[id]; dead {
			continue
		}
		dstPath := filepath.Join(target, "local_"+id+".json")
		var current *indexFile
		best := copies[0]
		for i := range copies {
			c := copies[i]
			if c.path == dstPath {
				current = &copies[i]
			}
			if c.lastActive > best.lastActive || (c.lastActive == best.lastActive && c.path == dstPath) {
				best = c
			}
		}
		if best.path == dstPath {
			continue
		}
		if current != nil && current.lastActive >= best.lastActive {
			continue
		}
		if current == nil && transcripts != nil && best.cliSession != "" && !transcripts[best.cliSession] {
			rep.SkippedDead++
			continue
		}
		if current != nil {
			if err := moveToBackup(backup, "sessions", dstPath); err != nil {
				return rep, err
			}
		}
		if err := copyFileAtomic(best.path, dstPath); err != nil {
			return rep, err
		}
		if current != nil {
			rep.Updated++
		} else {
			rep.Copied++
		}
	}
	return rep, nil
}

func readIndex(path string) indexFile {
	f := indexFile{path: path}
	file, err := os.Open(path)
	if err != nil {
		return f
	}
	defer file.Close()
	// Index files are mostly a large remote MCP config that comes after the
	// two fields needed here, so decoding stops once both have been seen.
	dec := json.NewDecoder(bufio.NewReader(file))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return f
	}
	var haveLast, haveCLI bool
	for dec.More() && !(haveLast && haveCLI) {
		t, err := dec.Token()
		if err != nil {
			return f
		}
		switch t {
		case "lastActivityAt":
			haveLast = dec.Decode(&f.lastActive) == nil
		case "cliSessionId":
			haveCLI = dec.Decode(&f.cliSession) == nil
		default:
			if dec.Decode(&json.RawMessage{}) != nil {
				return f
			}
		}
	}
	return f
}

// TranscriptIDs lists the session IDs that have a transcript under
// ~/.claude/projects, from file names only.
func TranscriptIDs(projectsDir string) (map[string]bool, error) {
	ids := map[string]bool{}
	projects, err := os.ReadDir(projectsDir)
	if errors.Is(err, os.ErrNotExist) {
		return ids, nil
	}
	if err != nil {
		return nil, err
	}
	for _, pr := range projects {
		if !pr.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(projectsDir, pr.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if name := f.Name(); strings.HasSuffix(name, ".jsonl") && !f.IsDir() {
				ids[strings.TrimSuffix(name, ".jsonl")] = true
			}
		}
	}
	return ids, nil
}

// moveToBackup moves path into <backup>/<sub>/ under a name that does not
// exist yet.
func moveToBackup(backup, sub, path string) error {
	dir := filepath.Join(backup, sub)
	dest := filepath.Join(dir, filepath.Base(path))
	for n := 2; exists(dest); n++ {
		dir = filepath.Join(backup, fmt.Sprintf("%s-%d", sub, n))
		dest = filepath.Join(dir, filepath.Base(path))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Rename(path, dest)
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".hotseat-tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return err
	}
	// Same mtime as the source lets later switches skip unchanged files
	// without reading them (see treesEqual).
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

// copyTree copies src to dst, keeping permissions and copying symlinks as
// symlinks.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		return copyFileAtomic(src, dst)
	}
}
