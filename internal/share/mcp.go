package share

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
)

// readServers returns the mcpServers map of a .claude.json. A missing file
// returns ok=false; a missing key returns an empty map.
func readServers(file string) (servers map[string]json.RawMessage, ok bool, err error) {
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", file, err)
	}
	if doc.MCPServers == nil {
		doc.MCPServers = map[string]json.RawMessage{}
	}
	return doc.MCPServers, true, nil
}

// mergedMCP starts from the default account's servers and adds servers only
// other accounts have. A name defined differently elsewhere keeps the
// default account's definition and is reported as a conflict.
func mergedMCP(files []string, defaultFile string) (map[string]json.RawMessage, []string, error) {
	merged := map[string]json.RawMessage{}
	var conflicts []string
	order := append([]string{defaultFile}, files...)
	seen := map[string]bool{}
	for _, f := range order {
		if seen[f] {
			continue
		}
		seen[f] = true
		servers, ok, err := readServers(f)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		for name, raw := range servers {
			existing, have := merged[name]
			if !have {
				merged[name] = raw
				continue
			}
			if !sameJSON(existing, raw) && !slices.Contains(conflicts, name) {
				conflicts = append(conflicts, name)
			}
		}
	}
	slices.Sort(conflicts)
	return merged, conflicts, nil
}

func sameServers(a, b map[string]json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for name, raw := range a {
		other, ok := b[name]
		if !ok || !sameJSON(raw, other) {
			return false
		}
	}
	return true
}

// sameJSON compares by meaning, so key order and escaping such as \u0026
// versus & do not count as a difference.
func sameJSON(a, b json.RawMessage) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(va, vb)
}

// writeServers replaces only the mcpServers key; every other key keeps its
// value. HTML escaping is off so values like URLs with & are written as-is.
func writeServers(file string, servers map[string]json.RawMessage, perm os.FileMode) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", file, err)
	}
	raw, err := encodeJSON(servers, "")
	if err != nil {
		return err
	}
	doc["mcpServers"] = bytes.TrimSpace(raw)
	out, err := encodeJSON(doc, "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(file, out, perm)
}

func encodeJSON(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
