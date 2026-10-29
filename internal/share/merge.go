package share

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// mergeJSONL unions two history.jsonl files without dropping any line:
// duplicates (ignoring surrounding whitespace) collapse to the first copy,
// and lines are stably sorted by their integer "timestamp" field, with lines
// that have none (or are not JSON) sorting as 0.
func mergeJSONL(core, acct []byte) []byte {
	type line struct {
		text string
		ts   int64
	}
	var lines []line
	seen := map[string]bool{}
	for _, src := range [][]byte{core, acct} {
		for _, raw := range strings.Split(string(src), "\n") {
			text := strings.TrimSuffix(raw, "\r")
			key := strings.TrimSpace(text)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			var rec struct {
				Timestamp int64 `json:"timestamp"`
			}
			if json.Unmarshal([]byte(key), &rec) != nil {
				rec.Timestamp = 0
			}
			lines = append(lines, line{text: text, ts: rec.Timestamp})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].ts < lines[j].ts })
	var buf bytes.Buffer
	for _, l := range lines {
		buf.WriteString(l.text)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// mergeTree copies into dst every entry of src that dst lacks, recursing into
// directories both sides have. Files present on both sides with different
// content keep dst's version and are reported as conflicts. onAdd is called
// with each destination path before it is copied, so a copy that fails
// halfway is still known to the caller.
func mergeTree(src, dst string, onAdd func(added string) error) (conflicts []string, err error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		dInfo, dErr := os.Lstat(d)
		if errors.Is(dErr, os.ErrNotExist) {
			if err := onAdd(d); err != nil {
				return conflicts, err
			}
			if err := copyTree(s, d); err != nil {
				return conflicts, err
			}
			continue
		}
		if dErr != nil {
			return conflicts, dErr
		}
		// An account entry that already links to the shared one is the
		// same content.
		if sameTarget(s, d) {
			continue
		}
		sInfo, err := os.Lstat(s)
		if err != nil {
			return conflicts, err
		}
		switch {
		case sInfo.IsDir() && dInfo.IsDir():
			c, err := mergeTree(s, d, onAdd)
			conflicts = append(conflicts, c...)
			if err != nil {
				return conflicts, err
			}
		case sInfo.Mode().IsRegular() && dInfo.Mode().IsRegular():
			same, err := sameFile(s, d)
			if err != nil {
				return conflicts, err
			}
			if !same {
				conflicts = append(conflicts, d)
			}
		case sInfo.Mode()&os.ModeSymlink != 0 && dInfo.Mode()&os.ModeSymlink != 0:
			st, _ := os.Readlink(s)
			dt, _ := os.Readlink(d)
			if st != dt {
				conflicts = append(conflicts, d)
			}
		default:
			conflicts = append(conflicts, d)
		}
	}
	return conflicts, nil
}

// copyTree copies src to dst, keeping permission bits and copying symlinks as
// symlinks rather than following them.
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
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
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
		return copyFile(src, dst, info.Mode().Perm())
	}
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeFileAtomic replaces path via a temp file in the same dir.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".hotseat-tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
