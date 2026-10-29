// Package settings edits ~/.claude/settings.json, a file people write by
// hand. Only the statusLine value and Hotseat's own StopFailure hook entries
// are rewritten; every other byte of the file is kept as it was.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Member is one key of a JSON object and the byte ranges of its key and
// value in the source.
type Member struct {
	Key                  string
	KeyStart             int
	ValueStart, ValueEnd int
}

// TopLevel lists the members of the object b holds.
func TopLevel(b []byte) ([]Member, error) {
	ms, _, err := parseObject(b)
	return ms, err
}

func parseObject(b []byte) ([]Member, int, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return nil, 0, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, 0, errors.New("settings: not a JSON object")
	}
	var out []Member
	for dec.More() {
		before := int(dec.InputOffset())
		tok, err := dec.Token()
		if err != nil {
			return nil, 0, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, 0, errors.New("settings: expected an object key")
		}
		afterKey := int(dec.InputOffset())
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, 0, err
		}
		end := int(dec.InputOffset())
		out = append(out, Member{
			Key:        key,
			KeyStart:   skip(b, before, ", \t\r\n"),
			ValueStart: skip(b, afterKey, ": \t\r\n"),
			ValueEnd:   end,
		})
	}
	if _, err := dec.Token(); err != nil {
		return nil, 0, err
	}
	return out, int(dec.InputOffset()) - 1, nil
}

func skip(b []byte, i int, chars string) int {
	for i < len(b) && strings.IndexByte(chars, b[i]) >= 0 {
		i++
	}
	return i
}

// indentOf returns the whitespace between the start of pos's line and pos.
func indentOf(b []byte, pos int) string {
	start := bytes.LastIndexByte(b[:pos], '\n') + 1
	ind := b[start:pos]
	if len(bytes.TrimLeft(ind, " \t")) != 0 {
		return ""
	}
	return string(ind)
}

func unit(b []byte, ms []Member) string {
	if len(ms) > 0 {
		if ind := indentOf(b, ms[0].KeyStart); ind != "" {
			return ind
		}
	}
	return "  "
}

func encode(v any, prefix, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func find(ms []Member, key string) int {
	for i, m := range ms {
		if m.Key == key {
			return i
		}
	}
	return -1
}

func splice(b []byte, start, end int, repl []byte) []byte {
	out := make([]byte, 0, len(b)-(end-start)+len(repl))
	out = append(out, b[:start]...)
	out = append(out, repl...)
	return append(out, b[end:]...)
}

// insertMember appends key: val after the last member of the object in b.
func insertMember(b []byte, ms []Member, closeBrace int, key string, val []byte, ind string) []byte {
	entry := strconv.Quote(key) + ": " + string(val)
	if len(ms) == 0 {
		return splice(b, closeBrace, closeBrace, []byte("\n"+ind+entry+"\n"+strings.TrimSuffix(ind, unitGuess(ind))))
	}
	last := ms[len(ms)-1]
	return splice(b, last.ValueEnd, last.ValueEnd, []byte(",\n"+ind+entry))
}

func unitGuess(ind string) string {
	if strings.HasPrefix(ind, "\t") {
		return "\t"
	}
	return "  "
}

// deleteMember removes member i, including the comma that joins it to its
// neighbour, so deleting what insertMember added restores the original.
func deleteMember(b []byte, ms []Member, i int) []byte {
	switch {
	case i > 0:
		return splice(b, ms[i-1].ValueEnd, ms[i].ValueEnd, nil)
	case len(ms) > 1:
		return splice(b, ms[0].KeyStart, ms[1].KeyStart, nil)
	default:
		return splice(b, ms[0].KeyStart, ms[0].ValueEnd, nil)
	}
}

// SetStatusLine points statusLine at command and returns the previous value
// (nil if there was none).
func SetStatusLine(file, command, backup string) (json.RawMessage, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	ms, closeBrace, err := parseObject(b)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	ind := unit(b, ms)
	val, err := encode(struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}{"command", command}, ind, ind)
	if err != nil {
		return nil, err
	}
	var prev json.RawMessage
	var nb []byte
	if i := find(ms, "statusLine"); i >= 0 {
		prev = append(json.RawMessage(nil), b[ms[i].ValueStart:ms[i].ValueEnd]...)
		nb = splice(b, ms[i].ValueStart, ms[i].ValueEnd, val)
	} else {
		nb = insertMember(b, ms, closeBrace, "statusLine", val, ind)
	}
	return prev, write(file, b, nb, backup)
}

// RestoreStatusLine puts back previous, or removes statusLine when previous
// is nil.
func RestoreStatusLine(file string, previous json.RawMessage, backup string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	ms, closeBrace, err := parseObject(b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}
	i := find(ms, "statusLine")
	var nb []byte
	switch {
	case previous == nil && i < 0:
		return nil
	case previous == nil:
		nb = deleteMember(b, ms, i)
	case i >= 0:
		nb = splice(b, ms[i].ValueStart, ms[i].ValueEnd, previous)
	default:
		nb = insertMember(b, ms, closeBrace, "statusLine", previous, unit(b, ms))
	}
	return write(file, b, nb, backup)
}

// StatusLineCommand returns the command of the current statusLine and its
// raw value.
func StatusLineCommand(file string) (string, json.RawMessage, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", nil, err
	}
	ms, _, err := parseObject(b)
	if err != nil {
		return "", nil, err
	}
	i := find(ms, "statusLine")
	if i < 0 {
		return "", nil, nil
	}
	raw := append(json.RawMessage(nil), b[ms[i].ValueStart:ms[i].ValueEnd]...)
	var sl struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(raw, &sl)
	return sl.Command, raw, nil
}

type hookEntry = map[string]any

// HookCommands lists the commands registered for event.
func HookCommands(file, event string) ([]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	var out []string
	for _, e := range doc.Hooks[event] {
		for _, h := range e.Hooks {
			out = append(out, h.Command)
		}
	}
	return out, nil
}

// AddHook registers command for event with matcher, once.
func AddHook(file, event, matcher, command, backup string) error {
	return editEvent(file, backup, event, func(entries []hookEntry) ([]hookEntry, bool) {
		for _, e := range entries {
			for _, h := range hooksOf(e) {
				if h["command"] == command {
					return entries, false
				}
			}
		}
		return append(entries, hookEntry{
			"matcher": matcher,
			"hooks":   []any{map[string]any{"type": "command", "command": command}},
		}), true
	})
}

// RemoveHook drops command from event; an event left empty is removed.
func RemoveHook(file, event, command, backup string) error {
	return editEvent(file, backup, event, func(entries []hookEntry) ([]hookEntry, bool) {
		changed := false
		var out []hookEntry
		for _, e := range entries {
			hs := hooksOf(e)
			var kept []any
			for _, h := range hs {
				if h["command"] == command {
					changed = true
					continue
				}
				kept = append(kept, h)
			}
			if len(kept) == 0 && len(hs) > 0 {
				continue
			}
			e["hooks"] = kept
			out = append(out, e)
		}
		return out, changed
	})
}

func hooksOf(e hookEntry) []map[string]any {
	list, _ := e["hooks"].([]any)
	var out []map[string]any
	for _, h := range list {
		if m, ok := h.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// editEvent rewrites only the value of hooks[event]; the rest of the hooks
// object and of the file keep their original bytes.
func editEvent(file, backup, event string, change func([]hookEntry) ([]hookEntry, bool)) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	ms, closeBrace, err := parseObject(b)
	if err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}
	top := unit(b, ms)
	hi := find(ms, "hooks")
	if hi < 0 {
		entries, changed := change(nil)
		if !changed || len(entries) == 0 {
			return nil
		}
		val, err := encode(map[string]any{event: entries}, top, top)
		if err != nil {
			return err
		}
		return write(file, b, insertMember(b, ms, closeBrace, "hooks", val, top), backup)
	}

	h := ms[hi]
	sub := b[h.ValueStart:h.ValueEnd]
	sms, sclose, err := parseObject(sub)
	if err != nil {
		return fmt.Errorf("parse hooks in %s: %w", file, err)
	}
	nested := indentOf(b, h.KeyStart) + top
	ei := find(sms, event)
	var entries []hookEntry
	if ei >= 0 {
		if err := json.Unmarshal(sub[sms[ei].ValueStart:sms[ei].ValueEnd], &entries); err != nil {
			return fmt.Errorf("parse hooks.%s in %s: %w", event, file, err)
		}
	}
	entries, changed := change(entries)
	if !changed {
		return nil
	}
	var newSub []byte
	switch {
	case len(entries) == 0 && ei >= 0:
		newSub = deleteMember(sub, sms, ei)
	case len(entries) == 0:
		return nil
	default:
		val, err := encode(entries, nested, top)
		if err != nil {
			return err
		}
		if ei >= 0 {
			newSub = splice(sub, sms[ei].ValueStart, sms[ei].ValueEnd, val)
		} else {
			newSub = insertMember(sub, sms, sclose, event, val, nested)
		}
	}
	return write(file, b, splice(b, h.ValueStart, h.ValueEnd, newSub), backup)
}

// write backs up the original bytes, then replaces the file (or the file a
// symlink points at) atomically, keeping its permissions.
func write(file string, orig, next []byte, backup string) error {
	if bytes.Equal(orig, next) {
		return nil
	}
	if !json.Valid(next) {
		return errors.New("settings: edit produced invalid JSON; nothing written")
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(backup, orig, 0o600); err != nil {
		return err
	}
	target := file
	if r, err := filepath.EvalSymlinks(file); err == nil {
		target = r
	}
	perm := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		perm = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".settings.hotseat-tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(next); err != nil {
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
