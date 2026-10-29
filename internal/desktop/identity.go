// Package desktop switches Claude Desktop between accounts. Each account has
// a complete Desktop data dir; the current one lives at the real path
// ~/Library/Application Support/Claude and the others are parked in the
// Hotseat store. Desktop keeps its own login inside each dir; Hotseat never
// reads or copies credentials.
package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// AccountUUID returns the account a Desktop data dir is logged in to. The
// same config.json holds Desktop's OAuth token cache, so it is scanned key
// by key and every other value is skipped without being kept.
func AccountUUID(dataDir string) (string, error) {
	f, err := os.Open(filepath.Join(dataDir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return "", errors.New("desktop: config.json is not a JSON object")
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		if tok == "lastKnownAccountUuid" {
			var id string
			if err := dec.Decode(&id); err != nil {
				return "", err
			}
			return id, nil
		}
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return "", err
		}
	}
	return "", nil
}
