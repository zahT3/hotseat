package share

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}
