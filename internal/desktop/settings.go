package desktop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// mirrored entries are replaced in dst by src's copy (dst's goes to backup).
var mirrored = []string{"Claude Extensions", "Claude Extensions Settings", "extensions-installations.json"}

// SyncSettings carries Desktop's own settings from src (the profile used
// last) to dst. It is a whitelist: login files (Cookies, config.json, Local
// Storage and the like) are never read or written.
func SyncSettings(src, dst, srcAcct, dstAcct, backup string) error {
	if err := syncDesktopConfig(src, dst, srcAcct, dstAcct, backup); err != nil {
		return fmt.Errorf("claude_desktop_config.json: %w", err)
	}
	if err := syncPresets(src, dst, srcAcct, dstAcct, backup); err != nil {
		return fmt.Errorf("ccd-session-presets.json: %w", err)
	}
	for _, name := range mirrored {
		s, d := filepath.Join(src, name), filepath.Join(dst, name)
		if !exists(s) {
			continue
		}
		if exists(d) {
			if err := moveToBackup(backup, "settings", d); err != nil {
				return err
			}
		}
		if err := copyTree(s, d); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return unionCopy(filepath.Join(src, "scratch-workspaces"), filepath.Join(dst, "scratch-workspaces"))
}

type object = map[string]json.RawMessage

func readObject(path string) (object, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return object{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	o := object{}
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, false, err
	}
	return o, true, nil
}

func writeObject(path string, o object, backup string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		return err
	}
	if exists(path) {
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, buf.Bytes()) {
			return nil
		}
		tmp := path + ".hotseat-backup"
		if err := copyFileAtomic(path, tmp); err != nil {
			return err
		}
		if err := moveToBackup(backup, "settings", tmp); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".hotseat-tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
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

// remap sets dst[dstAcct] to src[srcAcct], keeping dst's other accounts.
func remap(srcRaw, dstRaw json.RawMessage, srcAcct, dstAcct string) (json.RawMessage, error) {
	s, d := object{}, object{}
	if len(srcRaw) > 0 {
		if err := json.Unmarshal(srcRaw, &s); err != nil {
			return nil, err
		}
	}
	if len(dstRaw) > 0 {
		if err := json.Unmarshal(dstRaw, &d); err != nil {
			return nil, err
		}
	}
	if v, ok := s[srcAcct]; ok {
		d[dstAcct] = v
	}
	return json.Marshal(d)
}

func syncDesktopConfig(src, dst, srcAcct, dstAcct, backup string) error {
	name := "claude_desktop_config.json"
	s, ok, err := readObject(filepath.Join(src, name))
	if err != nil || !ok {
		return err
	}
	d, _, err := readObject(filepath.Join(dst, name))
	if err != nil {
		return err
	}
	out := object{}
	for k, v := range d {
		out[k] = v
	}
	for k, v := range s {
		if k != "preferences" {
			out[k] = v
		}
	}
	sp, dp := object{}, object{}
	if raw, ok := s["preferences"]; ok {
		if err := json.Unmarshal(raw, &sp); err != nil {
			return err
		}
	}
	if raw, ok := d["preferences"]; ok {
		if err := json.Unmarshal(raw, &dp); err != nil {
			return err
		}
	}
	prefs := object{}
	for k, v := range dp {
		prefs[k] = v
	}
	for k, v := range sp {
		if !strings.HasSuffix(k, "ByAccount") {
			prefs[k] = v
		}
	}
	keys := map[string]bool{}
	for k := range sp {
		keys[k] = true
	}
	for k := range dp {
		keys[k] = true
	}
	for k := range keys {
		if !strings.HasSuffix(k, "ByAccount") {
			continue
		}
		v, err := remap(sp[k], dp[k], srcAcct, dstAcct)
		if err != nil {
			return fmt.Errorf("preferences.%s: %w", k, err)
		}
		prefs[k] = v
	}
	raw, err := json.Marshal(prefs)
	if err != nil {
		return err
	}
	out["preferences"] = raw
	return writeObject(filepath.Join(dst, name), out, backup)
}

func syncPresets(src, dst, srcAcct, dstAcct, backup string) error {
	name := "ccd-session-presets.json"
	s, ok, err := readObject(filepath.Join(src, name))
	if err != nil || !ok {
		return err
	}
	d, _, err := readObject(filepath.Join(dst, name))
	if err != nil {
		return err
	}
	out := object{}
	for k, v := range d {
		out[k] = v
	}
	for k, v := range s {
		if k != "byAccount" {
			out[k] = v
		}
	}
	by, err := remap(s["byAccount"], d["byAccount"], srcAcct, dstAcct)
	if err != nil {
		return err
	}
	out["byAccount"] = by
	return writeObject(filepath.Join(dst, name), out, backup)
}

// unionCopy copies every file under src that dst lacks; existing files in
// dst are never overwritten.
func unionCopy(src, dst string) error {
	if !exists(src) {
		return nil
	}
	return filepath.WalkDir(src, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if e.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if exists(target) {
			return nil
		}
		return copyTree(path, target)
	})
}
