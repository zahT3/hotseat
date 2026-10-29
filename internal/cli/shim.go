package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zahT3/hotseat/internal/paths"
)

const (
	blockStart = "# >>> hotseat >>>"
	blockEnd   = "# <<< hotseat <<<"
)

func cmdShim(p paths.Paths, args []string, stdout, stderr io.Writer) int {
	jsonOut := hasFlag(args, "--json")
	args = positional(args)
	if len(args) != 1 || (args[0] != "install" && args[0] != "uninstall") {
		fmt.Fprintln(stderr, "usage: hotseat shim install|uninstall")
		return 1
	}
	if args[0] == "install" {
		if err := shimInstall(p); err != nil {
			return fail(stderr, err)
		}
		if jsonOut {
			return writeJSON(stdout, stderr, map[string]string{"shim": filepath.Join(p.ShimDir(), "claude"), "zshrc": p.Zshrc()})
		}
		fmt.Fprintf(stdout, "installed %s; open a new terminal so `claude` follows the current account\n", tildePath(p, filepath.Join(p.ShimDir(), "claude")))
		return 0
	}
	if err := shimUninstall(p); err != nil {
		return fail(stderr, err)
	}
	if jsonOut {
		return writeJSON(stdout, stderr, map[string]bool{"removed": true})
	}
	fmt.Fprintln(stdout, "removed the claude shim; open a new terminal")
	return 0
}

func shimInstall(p paths.Paths) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return err
	}
	if err := os.MkdirAll(p.ShimDir(), 0o755); err != nil {
		return err
	}
	link := filepath.Join(p.ShimDir(), "claude")
	if err := removeSymlink(link); err != nil {
		return err
	}
	if err := os.Symlink(self, link); err != nil {
		return err
	}

	content, err := readZshrc(p)
	if err != nil {
		return err
	}
	if err := backupZshrc(p, content); err != nil {
		return err
	}
	block := blockStart + "\n" + "export PATH=\"" + shellPath(p, p.ShimDir()) + ":$PATH\"\n" + blockEnd + "\n"
	if start, end, ok := findBlock(content); ok {
		content = content[:start] + block + content[end:]
	} else {
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += block
	}
	return writeZshrc(p, content)
}

func shimUninstall(p paths.Paths) error {
	if err := removeSymlink(filepath.Join(p.ShimDir(), "claude")); err != nil {
		return err
	}
	content, err := readZshrc(p)
	if err != nil {
		return err
	}
	start, end, ok := findBlock(content)
	if !ok {
		return nil
	}
	if err := backupZshrc(p, content); err != nil {
		return err
	}
	return writeZshrc(p, content[:start]+content[end:])
}

// findBlock returns the byte range of the hotseat block including its
// trailing newline.
func findBlock(content string) (start, end int, ok bool) {
	start = strings.Index(content, blockStart)
	if start < 0 {
		return 0, 0, false
	}
	rel := strings.Index(content[start:], blockEnd)
	if rel < 0 {
		return 0, 0, false
	}
	end = start + rel + len(blockEnd)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return start, end, true
}

func readZshrc(p paths.Paths) (string, error) {
	b, err := os.ReadFile(p.Zshrc())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// writeZshrc writes through a symlinked ~/.zshrc (dotfile managers) instead
// of replacing the link with a file.
func writeZshrc(p paths.Paths, content string) error {
	target := p.Zshrc()
	if r, err := filepath.EvalSymlinks(target); err == nil {
		target = r
	}
	perm := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		perm = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".zshrc.hotseat-tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
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
	return os.Rename(tmp.Name(), target)
}

func backupZshrc(p paths.Paths, content string) error {
	if err := os.MkdirAll(p.BackupsDir(), 0o700); err != nil {
		return err
	}
	name := filepath.Join(p.BackupsDir(), "zshrc-"+time.Now().Format("20060102-150405.000000"))
	return os.WriteFile(name, []byte(content), 0o600)
}

func removeSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a symlink; refusing to replace it", path)
	}
	return os.Remove(path)
}

func shellPath(p paths.Paths, path string) string {
	if strings.HasPrefix(path, p.Home+"/") {
		return "$HOME" + strings.TrimPrefix(path, p.Home)
	}
	return path
}
